package parser

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/fetch"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/workscope"
)

const InspectorResourceBytes = limits.MaxPageBytes
const InspectorPageBytes = 32 << 20
const InspectorRequestConcurrency = 8
const InspectorRequestQueue = 32

var errInspectorPageBudget = errors.New("inspector page response budget exceeded")
var errInspectorResourceBudget = errors.New("inspector resource response budget exceeded")
var errInspectorQueueBudget = errors.New("inspector request queue is full")

// inspectorBudget reserves chunks before reading so concurrent responses cannot
// retain more than the page's decoded-byte budget. Bytes consumed by failed
// requests still count against the page. An exhausted reader probes one byte
// only to distinguish an exact-boundary EOF from an oversized response.
type inspectorBudget struct {
	failure             error
	mu                  sync.Mutex
	remaining, reserved int64
	changed             chan struct{}
}

func newInspectorBudget() *inspectorBudget { return &inspectorBudget{remaining: InspectorPageBytes} }
func (b *inspectorBudget) reserve(ctx context.Context, want int) (int, bool, error) {
	for {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		b.mu.Lock()
		if b.remaining > 0 {
			amount := min(int64(want), b.remaining)
			b.remaining -= amount
			b.reserved += amount
			b.mu.Unlock()
			return int(amount), false, nil
		}
		if b.reserved == 0 {
			b.mu.Unlock()
			return 1, true, nil
		}
		if b.changed == nil {
			b.changed = make(chan struct{})
		}
		changed := b.changed
		b.mu.Unlock()
		select {
		case <-ctx.Done():
			return 0, false, ctx.Err()
		case <-changed:
		}
	}
}
func (b *inspectorBudget) finish(amount, n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reserved -= int64(amount)
	b.remaining += int64(amount - n)
	if b.changed != nil {
		close(b.changed)
		b.changed = nil
	}
}

func (b *inspectorBudget) record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failure == nil {
		b.failure = err
	}
}
func (b *inspectorBudget) Err() error { b.mu.Lock(); defer b.mu.Unlock(); return b.failure }

type inspectorReader struct {
	ctx    context.Context
	reader io.Reader
	budget *inspectorBudget
}

func (r inspectorReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	amount, probe, reserveErr := r.budget.reserve(r.ctx, len(p))
	if reserveErr != nil {
		return 0, reserveErr
	}
	n, err := r.reader.Read(p[:amount])
	if probe {
		if n > 0 {
			return 0, errInspectorPageBudget
		}
		return n, err
	}
	r.budget.finish(amount, n)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, canceled
	}
	return n, err
}

// readInspectorResponse owns the response body. Both compressed wire data and
// decoded content have per-resource limits; the shared page budget applies to
// decoded bytes. Header hints never replace counting the bytes actually read.
func readInspectorResponse(ctx context.Context, response *http.Response, budget *inspectorBudget) (data []byte, err error) {
	defer func() {
		if errors.Is(err, errInspectorResourceBudget) || errors.Is(err, errInspectorPageBudget) {
			budget.record(err)
		}
	}()
	if response.Body == nil {
		return nil, errors.New("inspector response has no body")
	}
	var once sync.Once
	closeBody := func() { once.Do(func() { _ = response.Body.Close() }) }
	stop := context.AfterFunc(ctx, closeBody)
	defer stop()
	defer closeBody()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if response.ContentLength > InspectorResourceBytes {
		return nil, fmt.Errorf("%w: %d bytes", errInspectorResourceBudget, InspectorResourceBytes)
	}
	wire := &io.LimitedReader{R: response.Body, N: InspectorResourceBytes + 1}
	var reader io.Reader = wire
	encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding")))
	if response.Uncompressed {
		encoding = ""
	}
	switch encoding {
	case "", "identity":
	case "gzip":
		decoded, err := gzip.NewReader(wire)
		if err != nil {
			if wire.N == 0 {
				return nil, fmt.Errorf("%w: compressed response", errInspectorResourceBudget)
			}
			return nil, err
		}
		defer decoded.Close()
		reader = decoded
		response.Header.Del("Content-Encoding")
		response.Header.Del("Content-Length")
		response.ContentLength = -1
		response.Uncompressed = true
	default:
		return nil, fmt.Errorf("unsupported inspector Content-Encoding %q", encoding)
	}
	body, err := io.ReadAll(io.LimitReader(inspectorReader{ctx, reader, budget}, InspectorResourceBytes+1))
	// A decoder can report unexpected EOF when the wire limiter truncates it.
	// Classify exhaustion first so the page cannot accept a partial capture.
	if wire.N == 0 {
		return nil, fmt.Errorf("%w: compressed response", errInspectorResourceBudget)
	}
	if err != nil {
		return nil, err
	}
	if len(body) > InspectorResourceBytes || wire.N == 0 {
		return nil, fmt.Errorf("%w: %d bytes", errInspectorResourceBudget, InspectorResourceBytes)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return body, nil
}

// A page owns a fixed set of interception workers and a bounded queue. No
// request event creates a goroutine, including rejected/blocked subresources.
type inspectorDispatcher struct {
	ctx           context.Context
	cancel        context.CancelFunc
	queue         chan *fetch.EventRequestPaused
	done, stopped chan struct{}
	mu            sync.Mutex
	closed        bool
	reject        func(*fetch.EventRequestPaused)
	rejections    sync.WaitGroup
}

func newInspectorDispatcher(ctx context.Context, handle func(context.Context, *fetch.EventRequestPaused), reject func(*fetch.EventRequestPaused)) *inspectorDispatcher {
	work, cancel := context.WithCancel(ctx)
	d := &inspectorDispatcher{ctx: work, cancel: cancel, queue: make(chan *fetch.EventRequestPaused, InspectorRequestQueue), done: make(chan struct{}), stopped: make(chan struct{}), reject: reject}
	workscope.Go(ctx, func() {
		defer close(d.done)
		defer cancel()
		var workers sync.WaitGroup
		workers.Add(InspectorRequestConcurrency)
		for range InspectorRequestConcurrency {
			go func() {
				defer workers.Done()
				for paused := range d.queue {
					if work.Err() == nil {
						handle(work, paused)
					}
				}
			}()
		}
		select {
		case <-work.Done():
			d.stop()
		case <-d.stopped:
		}
		workers.Wait()
		d.rejections.Wait()
	})
	return d
}
func (d *inspectorDispatcher) submit(paused *fetch.EventRequestPaused) bool {
	d.mu.Lock()
	if d.closed || d.ctx.Err() != nil {
		d.mu.Unlock()
		return false
	}
	select {
	case d.queue <- paused:
		d.mu.Unlock()
		return true
	default:
		// Register overload handling before admission can close, so final page
		// acceptance cannot race the callback that records its budget error.
		d.rejections.Add(1)
		d.mu.Unlock()
		defer d.rejections.Done()
		d.reject(paused)
		return false
	}
}
func (d *inspectorDispatcher) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.queue)
		close(d.stopped)
	}
}
func (d *inspectorDispatcher) finish() { d.stop(); <-d.done }
func (d *inspectorDispatcher) close()  { d.cancel(); d.stop(); <-d.done }

type inspectorInterception struct {
	*inspectorBudget
	dispatcher *inspectorDispatcher
}

func (i *inspectorInterception) finish() { i.dispatcher.finish() }

// result waits for admitted Inspector work before accepting a captured page.
// Deliberately blocked image/media/font work never records a budget failure.
func (i *inspectorInterception) result(actionErr error, drain bool) error {
	if drain {
		i.finish()
	}
	if actionErr != nil {
		return actionErr
	}
	return i.Err()
}
