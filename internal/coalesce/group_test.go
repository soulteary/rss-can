package coalesce

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type result struct {
	value  int
	err    error
	shared bool
}

func waiters(t *testing.T, g *Group[int], key string, n int) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for {
		g.mu.Lock()
		c := g.calls[key]
		ok := c != nil && c.waiters == n
		g.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(until) {
			t.Fatal("callers did not join shared work")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestSharedResultAndIndependentCancellation(t *testing.T) {
	var g Group[int]
	first, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	var calls atomic.Int32
	fn := func(ctx context.Context) (int, error) {
		calls.Add(1)
		select {
		case <-release:
			return 42, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	done := make(chan result, 2)
	go func() { v, e, s := g.Do(first, "key", time.Second, fn); done <- result{v, e, s} }()
	waiters(t, &g, "key", 1)
	go func() { v, e, s := g.Do(context.Background(), "key", time.Second, fn); done <- result{v, e, s} }()
	waiters(t, &g, "key", 2)
	cancel()
	if got := <-done; !errors.Is(got.err, context.Canceled) {
		t.Fatal(got)
	}
	close(release)
	if got := <-done; got.err != nil || got.value != 42 || !got.shared {
		t.Fatal(got)
	}
	if calls.Load() != 1 {
		t.Fatal("work duplicated")
	}
}
func TestAllCanceledFlightsCannotDeleteReplacement(t *testing.T) {
	var g Group[int]
	ctx, cancel := context.WithCancel(context.Background())
	oldRelease := make(chan struct{})
	oldDone := make(chan struct{})
	go func() {
		defer close(oldDone)
		g.Do(ctx, "key", time.Second, func(context.Context) (int, error) { <-oldRelease; return 1, nil })
	}()
	waiters(t, &g, "key", 1)
	g.mu.Lock()
	original := g.calls["key"]
	g.mu.Unlock()
	cancel()
	<-oldDone
	nextRelease := make(chan struct{})
	nextDone := make(chan result, 1)
	go func() {
		v, e, s := g.Do(context.Background(), "key", time.Second, func(context.Context) (int, error) { <-nextRelease; return 2, nil })
		nextDone <- result{v, e, s}
	}()
	waiters(t, &g, "key", 1)
	g.mu.Lock()
	replacement := g.calls["key"]
	g.mu.Unlock()
	close(oldRelease)
	// Wait for old work to finish; it must not forget its replacement.
	<-original.done
	g.mu.Lock()
	same := g.calls["key"] == replacement
	g.mu.Unlock()
	if !same {
		t.Fatal("old flight removed replacement")
	}
	close(nextRelease)
	if got := <-nextDone; got.err != nil || got.value != 2 {
		t.Fatal(got)
	}
}
func TestSharedWorkDeadlineAndErrorRetry(t *testing.T) {
	var g Group[int]
	_, err, _ := g.Do(context.Background(), "key", 20*time.Millisecond, func(ctx context.Context) (int, error) { <-ctx.Done(); return 0, ctx.Err() })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	v, err, shared := g.Do(context.Background(), "key", time.Second, func(context.Context) (int, error) { return 3, nil })
	if err != nil || shared || v != 3 {
		t.Fatalf("%v %v %v", v, err, shared)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err, _ = g.Do(ctx, "key", time.Second, func(context.Context) (int, error) { t.Fatal("canceled caller ran work"); return 0, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
