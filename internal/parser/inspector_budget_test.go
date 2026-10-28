package parser

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/fetch"
)

type inspectorCountBody struct {
	reader io.Reader
	reads  atomic.Int64
	closes atomic.Int32
}

func (b *inspectorCountBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.reads.Add(int64(n))
	return n, err
}
func (b *inspectorCountBody) Close() error { b.closes.Add(1); return nil }
func inspectorResponse(reader io.Reader, length int64, encoding string) (*http.Response, *inspectorCountBody) {
	body := &inspectorCountBody{reader: reader}
	header := make(http.Header)
	if encoding != "" {
		header.Set("Content-Encoding", encoding)
	}
	return &http.Response{StatusCode: 200, Body: body, Header: header, ContentLength: length}, body
}
func TestInspectorResourceBudgetCountsActualBytesAndCloses(t *testing.T) {
	value := strings.Repeat("x", InspectorResourceBytes+513)
	for _, test := range []struct {
		name      string
		data      string
		length    int64
		wantError bool
		maxRead   int64
	}{
		{"declared oversize", value, int64(len(value)), true, 0},
		{"unknown oversize", value, -1, true, InspectorResourceBytes + 1},
		{"lying length", value, 1, true, InspectorResourceBytes + 1},
		{"exact boundary", value[:InspectorResourceBytes], -1, false, InspectorResourceBytes},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, body := inspectorResponse(strings.NewReader(test.data), test.length, "")
			data, err := readInspectorResponse(t.Context(), response, newInspectorBudget())
			if (err != nil) != test.wantError {
				t.Fatalf("length=%d error=%v", len(data), err)
			}
			if !test.wantError && len(data) != InspectorResourceBytes {
				t.Fatalf("truncated valid response: %d", len(data))
			}
			if body.reads.Load() > test.maxRead || body.closes.Load() != 1 {
				t.Fatalf("read=%d close=%d", body.reads.Load(), body.closes.Load())
			}
		})
	}
}
func compressedInspectorBody(t *testing.T, value string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := io.WriteString(writer, value); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func TestInspectorBudgetCountsDecompressedBytes(t *testing.T) {
	for _, value := range []string{"small script", strings.Repeat("x", InspectorResourceBytes+1)} {
		compressed := compressedInspectorBody(t, value)
		response, body := inspectorResponse(bytes.NewReader(compressed), int64(len(compressed)), "gzip")
		response.Header.Set("Content-Length", "123")
		data, err := readInspectorResponse(t.Context(), response, newInspectorBudget())
		if len(value) > InspectorResourceBytes {
			if err == nil {
				t.Fatal("gzip expansion bypassed budget")
			}
		} else if err != nil || string(data) != value {
			t.Fatalf("%q %v", data, err)
		}
		if body.closes.Load() != 1 || !response.Uncompressed || response.Header.Get("Content-Encoding") != "" || response.Header.Get("Content-Length") != "" {
			t.Fatal("decoded response headers/body lifecycle incorrect")
		}
	}
	for _, encoding := range []string{"br", "gzip"} {
		response, body := inspectorResponse(strings.NewReader("invalid compressed body"), -1, encoding)
		if _, err := readInspectorResponse(t.Context(), response, newInspectorBudget()); err == nil || body.closes.Load() != 1 {
			t.Fatalf("%s %v", encoding, err)
		}
	}
	// Gzip header fields are not allowed to cause unbounded compressed reads.
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	writer.Name = strings.Repeat("x", InspectorResourceBytes+1)
	writer.Close()
	response, body := inspectorResponse(bytes.NewReader(buffer.Bytes()), -1, "gzip")
	if _, err := readInspectorResponse(t.Context(), response, newInspectorBudget()); err == nil || body.reads.Load() > InspectorResourceBytes+1 {
		t.Fatalf("gzip wire budget: %d %v", body.reads.Load(), err)
	}
}
func TestInspectorPageBudgetAcrossResources(t *testing.T) {
	budget := &inspectorBudget{remaining: 10}
	for _, value := range []string{"1234", "123456", ""} {
		response, _ := inspectorResponse(strings.NewReader(value), -1, "")
		data, err := readInspectorResponse(t.Context(), response, budget)
		if err != nil || string(data) != value {
			t.Fatalf("%q %v", data, err)
		}
	}
	response, body := inspectorResponse(strings.NewReader("x"), -1, "")
	if _, err := readInspectorResponse(t.Context(), response, budget); !errors.Is(err, errInspectorPageBudget) || body.reads.Load() != 1 {
		t.Fatalf("page budget %d %v", body.reads.Load(), err)
	}
}
func TestInspectorReservationsWaitForActualConsumption(t *testing.T) {
	budget := &inspectorBudget{remaining: 5}
	amount, probe, err := budget.reserve(t.Context(), 5)
	if err != nil || probe || amount != 5 {
		t.Fatal(amount, probe, err)
	}
	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(entered)
		amount, probe, err := budget.reserve(t.Context(), 1)
		if err == nil && probe {
			err = errors.New("reserved bytes mistaken for consumed bytes")
		}
		if err == nil {
			budget.finish(amount, 1)
		}
		done <- err
	}()
	<-entered
	select {
	case err := <-done:
		t.Fatalf("reservation did not wait: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	budget.finish(5, 2)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	amount, _, _ = budget.reserve(t.Context(), 2)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := budget.reserve(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	budget.finish(amount, 0)
}
func TestInspectorCanceledResponseClosesWithoutReading(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	response, body := inspectorResponse(strings.NewReader("unread"), -1, "")
	if _, err := readInspectorResponse(ctx, response, newInspectorBudget()); !errors.Is(err, context.Canceled) || body.reads.Load() != 0 || body.closes.Load() != 1 {
		t.Fatalf("%d %d %v", body.reads.Load(), body.closes.Load(), err)
	}
	if _, err := readInspectorResponse(t.Context(), &http.Response{}, newInspectorBudget()); err == nil {
		t.Fatal("missing body accepted")
	}
	var reader inspectorReader
	if n, err := reader.Read(nil); n != 0 || err != nil {
		t.Fatal(n, err)
	}
}
func TestInspectorDispatcherBoundsConcurrencyAndQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{}, InspectorRequestConcurrency)
	var active, maximum, rejected atomic.Int32
	dispatcher := newInspectorDispatcher(ctx, func(ctx context.Context, _ *fetch.EventRequestPaused) {
		current := active.Add(1)
		defer active.Add(-1)
		for previous := maximum.Load(); current > previous && !maximum.CompareAndSwap(previous, current); previous = maximum.Load() {
		}
		started <- struct{}{}
		<-ctx.Done()
	}, func(*fetch.EventRequestPaused) { rejected.Add(1) })
	defer dispatcher.close()
	for range InspectorRequestConcurrency {
		if !dispatcher.submit(&fetch.EventRequestPaused{}) {
			t.Fatal("active work rejected")
		}
	}
	for range InspectorRequestConcurrency {
		<-started
	}
	for range InspectorRequestQueue {
		if !dispatcher.submit(&fetch.EventRequestPaused{}) {
			t.Fatal("queue capacity incorrect")
		}
	}
	if dispatcher.submit(&fetch.EventRequestPaused{}) || rejected.Load() != 1 {
		t.Fatal("full queue admitted work")
	}
	dispatcher.close()
	if active.Load() != 0 || maximum.Load() != InspectorRequestConcurrency {
		t.Fatalf("active=%d max=%d", active.Load(), maximum.Load())
	}
	if dispatcher.submit(&fetch.EventRequestPaused{}) {
		t.Fatal("closed dispatcher admitted work")
	}
}

type inspectorBlockingBody struct {
	entered chan struct{}
	closed  chan struct{}
	once    sync.Once
	closes  atomic.Int32
}

func (b *inspectorBlockingBody) Read([]byte) (int, error) {
	close(b.entered)
	<-b.closed
	return 0, io.ErrClosedPipe
}
func (b *inspectorBlockingBody) Close() error {
	b.closes.Add(1)
	b.once.Do(func() { close(b.closed) })
	return nil
}
func TestInspectorCancellationInterruptsBodyRead(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	body := &inspectorBlockingBody{entered: make(chan struct{}), closed: make(chan struct{})}
	response := &http.Response{Body: body, Header: make(http.Header), ContentLength: -1}
	done := make(chan error, 1)
	go func() { _, err := readInspectorResponse(ctx, response, newInspectorBudget()); done <- err }()
	<-body.entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled body remained blocked")
	}
	if body.closes.Load() != 1 {
		t.Fatalf("close count=%d", body.closes.Load())
	}
}

func TestInspectorBudgetFailureRejectsOtherwiseSuccessfulPage(t *testing.T) {
	budget := newInspectorBudget()
	response, body := inspectorResponse(strings.NewReader("unread"), InspectorResourceBytes+1, "")
	if _, err := readInspectorResponse(t.Context(), response, budget); !errors.Is(err, errInspectorResourceBudget) {
		t.Fatal(err)
	}
	dispatcher := newInspectorDispatcher(t.Context(), func(context.Context, *fetch.EventRequestPaused) {}, func(*fetch.EventRequestPaused) {})
	defer dispatcher.close()
	control := &inspectorInterception{inspectorBudget: budget, dispatcher: dispatcher}
	if err := control.result(nil, true); !errors.Is(err, errInspectorResourceBudget) {
		t.Fatalf("partial page was accepted: %v", err)
	}
	cause := errors.New("page action failed")
	if err := control.result(cause, true); !errors.Is(err, cause) {
		t.Fatal("action error replaced")
	}
	if body.closes.Load() != 1 {
		t.Fatal("rejected resource left open")
	}
}
func TestInspectorDispatcherDrainsAdmittedWork(t *testing.T) {
	var handled atomic.Int32
	dispatcher := newInspectorDispatcher(t.Context(), func(context.Context, *fetch.EventRequestPaused) { handled.Add(1) }, func(*fetch.EventRequestPaused) { t.Error("unexpected overload") })
	for range InspectorRequestConcurrency {
		if !dispatcher.submit(&fetch.EventRequestPaused{}) {
			t.Fatal("work rejected")
		}
	}
	dispatcher.finish()
	if handled.Load() != InspectorRequestConcurrency {
		t.Fatalf("accepted work not joined: %d", handled.Load())
	}
	dispatcher.close()
}

func TestInspectorFinalAcceptanceJoinsOverloadCallback(t *testing.T) {
	workRelease, rejectRelease := make(chan struct{}), make(chan struct{})
	started := make(chan struct{}, InspectorRequestConcurrency)
	rejectStarted := make(chan struct{})
	var workOnce, rejectOnce sync.Once
	releaseWork := func() { workOnce.Do(func() { close(workRelease) }) }
	releaseReject := func() { rejectOnce.Do(func() { close(rejectRelease) }) }
	var calls atomic.Int32
	dispatcher := newInspectorDispatcher(t.Context(), func(ctx context.Context, _ *fetch.EventRequestPaused) {
		if calls.Add(1) <= InspectorRequestConcurrency {
			started <- struct{}{}
		}
		select {
		case <-workRelease:
		case <-ctx.Done():
		}
	}, func(*fetch.EventRequestPaused) { close(rejectStarted); <-rejectRelease })
	defer func() { releaseWork(); releaseReject(); dispatcher.close() }()
	for range InspectorRequestConcurrency {
		dispatcher.submit(&fetch.EventRequestPaused{})
	}
	for range InspectorRequestConcurrency {
		<-started
	}
	for range InspectorRequestQueue {
		dispatcher.submit(&fetch.EventRequestPaused{})
	}
	overflow := make(chan bool, 1)
	go func() { overflow <- dispatcher.submit(&fetch.EventRequestPaused{}) }()
	<-rejectStarted
	finished := make(chan struct{})
	go func() { dispatcher.finish(); close(finished) }()
	releaseWork()
	select {
	case <-finished:
		t.Fatal("page accepted before overload outcome completed")
	case <-time.After(10 * time.Millisecond):
	}
	releaseReject()
	if <-overflow {
		t.Fatal("overload admitted")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("overload cleanup was not joined")
	}
}
