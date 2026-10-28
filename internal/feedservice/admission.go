package feedservice

import (
	"context"
	"errors"
	"sync"
)

const GenerationConcurrency = 8
const GenerationQueueLimit = 32

var ErrOverloaded = errors.New("feed generation queue is full")

type admission struct{ admitted, active chan struct{} }

func newAdmission(active, queued int) *admission {
	return &admission{make(chan struct{}, active+queued), make(chan struct{}, active)}
}
func (a *admission) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case a.admitted <- struct{}{}:
	default:
		return nil, ErrOverloaded
	}
	select {
	case a.active <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-a.active
			<-a.admitted
			return nil, err
		}
		var once sync.Once
		return func() { once.Do(func() { <-a.active; <-a.admitted }) }, nil
	case <-ctx.Done():
		<-a.admitted
		return nil, ctx.Err()
	}
}
