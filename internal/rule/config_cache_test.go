package rule

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestConfigCacheContentExpiryReloadAndBound(t *testing.T) {
	var store configStore
	var count int
	evaluate := func(_ context.Context, r define.RuleCache) (define.JavaScriptConfig, error) {
		count++
		return define.JavaScriptConfig{Name: fmt.Sprint(count), File: r.File}, nil
	}
	rule := define.RuleCache{File: "rule.js", Body: []byte("first")}
	load := func() define.JavaScriptConfig {
		t.Helper()
		v, e := store.load(context.Background(), rule, time.Hour, evaluate)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	if a, b := load(), load(); a.Name != b.Name || count != 1 {
		t.Fatal("cache missed")
	}
	rule.Body = []byte("second")
	if load().Name != "2" {
		t.Fatal("content edit reused config")
	}
	store.mu.Lock()
	for k, e := range store.entries {
		e.expires = time.Now().Add(-time.Second)
		store.entries[k] = e
	}
	store.mu.Unlock()
	if load().Name != "3" {
		t.Fatal("expired configuration reused")
	}
	store.clear()
	if load().Name != "4" {
		t.Fatal("reload reused old evaluation")
	}
	for i := range 260 {
		rule.Body = []byte(fmt.Sprint(i))
		load()
	}
	if len(store.entries) > 256 {
		t.Fatal("unbounded cache")
	}
}
func TestConfigCacheCoalescesAndDoesNotCacheErrors(t *testing.T) {
	var store configStore
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	evaluate := func(ctx context.Context, _ define.RuleCache) (define.JavaScriptConfig, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return define.JavaScriptConfig{Name: "one"}, nil
		case <-ctx.Done():
			return define.JavaScriptConfig{}, ctx.Err()
		}
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			v, e := store.load(context.Background(), define.RuleCache{}, time.Minute, evaluate)
			if e != nil || v.Name != "one" {
				t.Errorf("%#v %v", v, e)
			}
		})
	}
	<-entered
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("evaluations=%d", calls.Load())
	}
	store.clear()
	fail := func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
		return define.JavaScriptConfig{}, errors.New("bad config")
	}
	for range 2 {
		if _, e := store.load(context.Background(), define.RuleCache{}, time.Minute, fail); e == nil {
			t.Fatal("error lost")
		}
	}
	if len(store.entries) != 0 {
		t.Fatal("failed config cached")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := store.load(ctx, define.RuleCache{}, time.Minute, fail); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestConfigCacheTTLAndDisabledDynamicEvaluation(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want time.Duration
	}{{"", 0}, {"bad", 0}, {"-1", 0}, {"0", 0}, {"30", 30 * time.Second}, {"99999", time.Hour}} {
		t.Setenv("RSS_CONFIG_CACHE_TTL", tc.env)
		if got := configCacheTTL(); got != tc.want {
			t.Fatalf("%s: %v", tc.env, got)
		}
	}
	var s configStore
	calls := 0
	fn := func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
		calls++
		return define.JavaScriptConfig{}, nil
	}
	for range 2 {
		s.load(context.Background(), define.RuleCache{}, 0, fn)
	}
	if calls != 2 {
		t.Fatal("dynamic config unexpectedly cached")
	}
}
func TestConfigReloadDuringEvaluationDoesNotRepopulate(t *testing.T) {
	var store configStore
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		store.load(context.Background(), define.RuleCache{}, time.Minute, func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
			close(entered)
			<-release
			return define.JavaScriptConfig{}, nil
		})
	}()
	<-entered
	store.clear()
	close(release)
	<-done
	if len(store.entries) != 0 {
		t.Fatal("reload repopulated stale config")
	}
}
