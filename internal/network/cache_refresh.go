package network

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
)

type refreshCacheKey struct{}
type documentRevisionKey struct{}

// WithCacheRefresh bypasses reads but replaces successful cached documents.
// Unlike DisableCache, it preserves caching for subsequent ordinary requests.
func WithCacheRefresh(ctx context.Context) context.Context {
	return context.WithValue(ctx, refreshCacheKey{}, true)
}
func CacheRefresh(ctx context.Context) bool {
	value, _ := ctx.Value(refreshCacheKey{}).(bool)
	return value
}

// WithDocumentRevision isolates managed source revisions without retaining their content.
func WithDocumentRevision(ctx context.Context, revision []byte) context.Context {
	return context.WithValue(ctx, documentRevisionKey{}, sha256.Sum256(revision))
}
func DocumentCacheKey(ctx context.Context, key string) string {
	if revision, ok := ctx.Value(documentRevisionKey{}).([32]byte); ok {
		return fmt.Sprintf("%s:revision:%x", key, revision)
	}
	return key
}

var cacheWrites = struct {
	sync.Mutex
	active map[string]*cacheWriteState
}{active: make(map[string]*cacheWriteState)}

type cacheWriteState struct {
	gate            chan struct{}
	refs            int
	next, committed uint64
}

// BeginCacheWrite orders overlapping successful writes by fetch start. State is
// retained only while fetches are active, so distinct source URLs cannot grow it.
func BeginCacheWrite(ctx context.Context, key string) (commit func(func() error) error, finish func()) {
	cacheWrites.Lock()
	state := cacheWrites.active[key]
	if state == nil {
		state = &cacheWriteState{gate: make(chan struct{}, 1)}
		cacheWrites.active[key] = state
	}
	state.refs++
	state.next++
	sequence := state.next
	cacheWrites.Unlock()
	return func(write func() error) error {
			select {
			case state.gate <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			defer func() { <-state.gate }()
			if err := ctx.Err(); err != nil {
				return err
			}
			if sequence < state.committed {
				return nil
			}
			if err := write(); err != nil {
				return err
			}
			state.committed = sequence
			return nil
		}, func() {
			cacheWrites.Lock()
			defer cacheWrites.Unlock()
			state.refs--
			if state.refs == 0 {
				delete(cacheWrites.active, key)
			}
		}
}
