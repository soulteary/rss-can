package parser

import (
	"context"
	"errors"
	"sync"
)

// BrowserConcurrency and BrowserQueueLimit are shared by feeds and Inspector.
// They bound Chrome pressure for the whole process, including separate Apps.
const (
	BrowserConcurrency = 4
	BrowserQueueLimit  = 16
)

var ErrBrowserOverloaded = errors.New("browser queue is full")

// Admission is intentionally shared: giving Inspector a separate budget would
// allow management traffic to exceed the total Chrome concurrency limit.
var browserQueue = newBrowserAdmission(BrowserConcurrency, BrowserQueueLimit)

// Keep the active slot channel for existing internal cancellation tests.
var browserSlots = browserQueue.active

type browserAdmission struct{ admitted, active chan struct{} }

func newBrowserAdmission(concurrency, queued int) *browserAdmission {
	return &browserAdmission{
		admitted: make(chan struct{}, concurrency+queued),
		active:   make(chan struct{}, concurrency),
	}
}

func (b *browserAdmission) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case b.admitted <- struct{}{}:
	default:
		return nil, ErrBrowserOverloaded
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
