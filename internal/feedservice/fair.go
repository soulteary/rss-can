package feedservice

import (
	"context"
	"errors"
	"sync"
)

var ErrRateLimited = errors.New("feed owner queue is full")

// fairAdmission bounds both total and per-owner demand. Waiting owners take
// turns; adding more requests for one owner cannot move it ahead of another.
type fairAdmission struct {
	mu                                             sync.Mutex
	limit, queueLimit, ownerLimit, ownerQueueLimit int
	active, queued                                 int
	owners                                         map[string]*fairOwner
	order                                          []string
}
type fairOwner struct {
	active  int
	waiting []*fairWaiter
}
type fairWaiter struct {
	ready   chan struct{}
	granted bool
}

func newFairAdmission(active, queued, ownerActive, ownerQueued int) *fairAdmission {
	return &fairAdmission{limit: active, queueLimit: queued, ownerLimit: ownerActive, ownerQueueLimit: ownerQueued, owners: make(map[string]*fairOwner)}
}
func (a *fairAdmission) acquire(ctx context.Context, key string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	o := a.owners[key]
	if o == nil {
		o = &fairOwner{}
		a.owners[key] = o
	}
	if a.active < a.limit && o.active < a.ownerLimit && len(a.order) == 0 {
		a.active++
		o.active++
		a.mu.Unlock()
		release := a.release(key)
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		return release, nil
	}
	if a.queued >= a.queueLimit || len(o.waiting) >= a.ownerQueueLimit {
		err := ErrOverloaded
		if len(o.waiting) >= a.ownerQueueLimit {
			err = ErrRateLimited
		}
		if o.active == 0 && len(o.waiting) == 0 {
			delete(a.owners, key)
		}
		a.mu.Unlock()
		return nil, err
	}
	w := &fairWaiter{ready: make(chan struct{})}
	if len(o.waiting) == 0 {
		a.order = append(a.order, key)
	}
	o.waiting = append(o.waiting, w)
	a.queued++
	a.dispatch()
	a.mu.Unlock()
	select {
	case <-w.ready:
		if err := ctx.Err(); err != nil {
			a.release(key)()
			return nil, err
		}
		return a.release(key), nil
	case <-ctx.Done():
		a.mu.Lock()
		if w.granted {
			a.mu.Unlock()
			a.release(key)()
			return nil, ctx.Err()
		}
		o := a.owners[key]
		for i, pending := range o.waiting {
			if pending == w {
				o.waiting = append(o.waiting[:i], o.waiting[i+1:]...)
				a.queued--
				break
			}
		}
		if len(o.waiting) == 0 {
			a.removeOrder(key)
			if o.active == 0 {
				delete(a.owners, key)
			}
		}
		a.dispatch()
		a.mu.Unlock()
		return nil, ctx.Err()
	}
}
func (a *fairAdmission) release(key string) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			o := a.owners[key]
			o.active--
			a.active--
			if o.active == 0 && len(o.waiting) == 0 {
				delete(a.owners, key)
			}
			a.dispatch()
		})
	}
}
func (a *fairAdmission) removeOrder(key string) {
	for i, k := range a.order {
		if k == key {
			a.order = append(a.order[:i], a.order[i+1:]...)
			return
		}
	}
}
func (a *fairAdmission) dispatch() {
	for a.active < a.limit && len(a.order) > 0 {
		chosen := -1
		for i, key := range a.order {
			if a.owners[key].active < a.ownerLimit {
				chosen = i
				break
			}
		}
		if chosen < 0 {
			return
		}
		key := a.order[chosen]
		a.order = append(a.order[:chosen], a.order[chosen+1:]...)
		o := a.owners[key]
		w := o.waiting[0]
		o.waiting = o.waiting[1:]
		a.queued--
		a.active++
		o.active++
		w.granted = true
		if len(o.waiting) > 0 {
			a.order = append(a.order, key)
		}
		close(w.ready)
	}
}
