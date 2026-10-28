package network

import (
	"context"
	"errors"
	"io"
	"sync"
)

// FetchConcurrency and FetchQueueLimit bound process-wide upstream HTTP work.
// Cached documents and callers sharing an in-flight fetch do not occupy slots.
const (
	FetchConcurrency = 16
	FetchQueueLimit  = 64
)

var ErrFetchOverloaded = errors.New("upstream fetch queue is full")

var upstreamBudget = newFetchBudget(FetchConcurrency, FetchQueueLimit)

type fetchBudget struct {
	admitted chan struct{}
	active   chan struct{}
}

func newFetchBudget(concurrency, queued int) *fetchBudget {
	return &fetchBudget{
		admitted: make(chan struct{}, concurrency+queued),
		active:   make(chan struct{}, concurrency),
	}
}

func (b *fetchBudget) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case b.admitted <- struct{}{}:
	default:
		return nil, ErrFetchOverloaded
	}
	select {
	case b.active <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-b.active
			<-b.admitted
			return nil, err
		}
		var once sync.Once
		return func() { once.Do(func() { <-b.active; <-b.admitted }) }, nil
	case <-ctx.Done():
		<-b.admitted
		return nil, ctx.Err()
	}
}

// A request retains its slot until its response body is consumed or closed,
// rather than releasing the slot when only the response headers have arrived.
type budgetBody struct {
	io.ReadCloser
	release func()
}

func (b *budgetBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.release()
	}
	return n, err
}

func (b *budgetBody) Close() error {
	defer b.release()
	return b.ReadCloser.Close()
}
