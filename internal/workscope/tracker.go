// Package workscope joins application-owned asynchronous cleanup after callers
// have stopped admitting work and drained their request handlers.
package workscope

import (
	"context"
	"sync"
)

// Tracker owns callback lifetimes, including work that outlives its last waiter.
// Wait must only be called after caller admission is closed and all callers
// that could register their first callback have drained. Nested work may still
// register callbacks while another tracked callback is active.
type Tracker struct{ work sync.WaitGroup }
type trackerKey struct{}

func WithTracker(ctx context.Context, tracker *Tracker) context.Context {
	return context.WithValue(ctx, trackerKey{}, tracker)
}

// Go registers before launching so a returning/canceled caller cannot race its
// owner's subsequent Wait. Without a tracker it retains ordinary goroutine
// behavior for legacy callers.
func Go(ctx context.Context, callback func()) {
	tracker, _ := ctx.Value(trackerKey{}).(*Tracker)
	if tracker == nil {
		go callback()
		return
	}
	tracker.work.Add(1)
	go func() { defer tracker.work.Done(); callback() }()
}

func (t *Tracker) Wait() { t.work.Wait() }
