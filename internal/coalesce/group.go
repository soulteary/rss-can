// Package coalesce shares in-flight work while keeping each caller cancelable.
package coalesce

import (
	"context"
	"sync"
	"time"
)

type call[T any] struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	value   T
	err     error
}

type Group[T any] struct {
	mu    sync.Mutex
	calls map[string]*call[T]
}

// Do gives shared work its own bounded context. One departing caller cannot
// cancel others; when the last caller leaves, work is canceled and forgotten.
func (g *Group[T]) Do(ctx context.Context, key string, timeout time.Duration, fn func(context.Context) (T, error)) (T, error, bool) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err, false
	}
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*call[T])
	}
	c, shared := g.calls[key]
	if !shared {
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		c = &call[T]{done: make(chan struct{}), cancel: cancel}
		g.calls[key] = c
		go func() {
			defer cancel()
			value, err := fn(workCtx)
			g.mu.Lock()
			c.value, c.err = value, err
			if g.calls[key] == c {
				delete(g.calls, key)
			}
			close(c.done)
			g.mu.Unlock()
		}()
	}
	c.waiters++
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		c.waiters--
		if c.waiters == 0 {
			c.cancel()
			if g.calls[key] == c {
				delete(g.calls, key)
			}
		}
	}()
	select {
	case <-ctx.Done():
		return zero, ctx.Err(), shared
	case <-c.done:
		if err := ctx.Err(); err != nil {
			return zero, err, shared
		}
		return c.value, c.err, shared
	}
}
