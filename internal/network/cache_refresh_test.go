package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestRefreshWinsOverOlderInflightCacheWrite(t *testing.T) {
	logger.Initialize()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			_, _ = w.Write([]byte("old"))
			return
		}
		_, _ = w.Write([]byte("new"))
	}))
	defer server.Close()
	cache := cacher.New(cacher.Config{MemoryEnabled: true, MemoryTTL: time.Hour})
	defer cache.Close()
	ctx := cacher.WithCache(t.Context(), cache)
	done := make(chan define.RemoteBodySanitized, 1)
	go func() { done <- GetRemoteDocumentContext(ctx, server.URL, "", 3600, false) }()
	<-started
	fresh := GetRemoteDocumentContext(WithCacheRefresh(ctx), server.URL, "", 3600, false)
	close(release)
	old := <-done
	if fresh.Body != "new" || old.Body != "old" {
		t.Fatalf("fresh=%q old=%q", fresh.Body, old.Body)
	}
	cached := GetRemoteDocumentContext(ctx, server.URL, "", 3600, false)
	if cached.Body != "new" || calls.Load() != 2 {
		t.Fatalf("cache=%q calls=%d", cached.Body, calls.Load())
	}
}

func TestDocumentRevisionSeparatesCacheAndRefreshFailurePreservesIt(t *testing.T) {
	logger.Initialize()
	var calls atomic.Int64
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if fail.Load() {
			w.WriteHeader(500)
			return
		}
		if n == 1 {
			_, _ = w.Write([]byte("one"))
		} else {
			_, _ = w.Write([]byte("two"))
		}
	}))
	defer server.Close()
	cache := cacher.New(cacher.Config{MemoryEnabled: true, MemoryTTL: time.Hour})
	defer cache.Close()
	ctx := cacher.WithCache(t.Context(), cache)
	first := WithDocumentRevision(ctx, []byte("revision1"))
	second := WithDocumentRevision(ctx, []byte("revision2"))
	if got := GetRemoteDocumentContext(first, server.URL, "", 3600, false); got.Body != "one" {
		t.Fatal(got)
	}
	if got := GetRemoteDocumentContext(second, server.URL, "", 3600, false); got.Body != "two" {
		t.Fatal(got)
	}
	fail.Store(true)
	if got := GetRemoteDocumentContext(WithCacheRefresh(second), server.URL, "", 3600, false); got.Code == define.ERROR_CODE_NULL {
		t.Fatal("refresh should fail")
	}
	if got := GetRemoteDocumentContext(second, server.URL, "", 3600, false); got.Body != "two" {
		t.Fatal(got)
	}
	if got := GetRemoteDocumentContext(first, server.URL, "", 3600, false); got.Body != "one" {
		t.Fatal(got)
	}
	if DocumentCacheKey(context.Background(), "key") != "key" {
		t.Fatal("legacy key changed")
	}
}

func TestCacheWriteAdmissionDoesNotBlockOtherKeysAndHonorsCancellation(t *testing.T) {
	commit, finish := BeginCacheWrite(t.Context(), t.Name())
	defer finish()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() { done <- commit(func() error { close(entered); <-release; return nil }) }()
	<-entered
	defer close(release)
	// Same-key admission must not hold the registry while waiting for a write.
	admitted := make(chan struct{})
	nextDone := make(chan struct{})
	canceled, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		defer close(nextDone)
		next, nextFinish := BeginCacheWrite(canceled, t.Name())
		defer nextFinish()
		other, otherFinish := BeginCacheWrite(t.Context(), t.Name()+"-other")
		defer otherFinish()
		if err := other(func() error { return nil }); err != nil {
			t.Error(err)
		}
		close(admitted)
		cancel()
		if err := next(func() error { t.Error("canceled write executed"); return nil }); err != context.Canceled {
			t.Errorf("canceled write: %v", err)
		}
	}()
	select {
	case <-admitted:
	case <-time.After(time.Second):
		t.Fatal("unrelated key blocked by slow cache write")
	}
	cancel()
	select {
	case <-nextDone:
	case <-time.After(time.Second):
		t.Fatal("canceled cache write did not return")
	}
	// Unblock the first commit and ensure its goroutine completes on test exit.
	t.Cleanup(func() {
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
}
