package feedservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
)

func TestManualRefreshFetchesAndReplacesUpstreamCache(t *testing.T) {
	logger.Initialize()
	var calls atomic.Int64
	var body atomic.Value
	body.Store("first")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	defer upstream.Close()
	cache := cacher.New(cacher.Config{MemoryEnabled: true, MemoryTTL: time.Hour})
	defer cache.Close()
	ctx := cacher.WithCache(t.Context(), cache)
	d := Dependencies{
		Lookup: func(string) (define.RuleCache, bool) { return define.RuleCache{Body: []byte("rule")}, true },
		Configure: func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
			return define.JavaScriptConfig{URL: upstream.URL, Expire: 3600}, nil
		},
		Fetch: func(ctx context.Context, c define.JavaScriptConfig) define.BodyParsed {
			doc := network.GetRemoteDocumentContext(ctx, c.URL, "", c.Expire, c.DisableCache)
			return define.BodyParsed{Code: doc.Code, Status: doc.Status, Body: []define.InfoItem{{Title: doc.Body}}}
		},
		Serialize: func(_ context.Context, _ define.JavaScriptConfig, d define.BodyParsed, _ string) (string, error) {
			return d.Body[0].Title, nil
		},
	}
	snapshots, err := NewSnapshots(ctx, New(time.Second, d), SnapshotOptions{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshots.Close()
	first, err := snapshots.Representation(ctx, "source", "rss")
	if err != nil {
		t.Fatal(err)
	}
	body.Store("updated")
	refreshed, err := snapshots.Refresh(ctx, "source", "rss")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Body != "updated" || calls.Load() != 2 {
		t.Fatalf("manual refresh served %q (initial=%q), upstream requests=%d; expected updated body and a second upstream request", refreshed.Body, first.Body, calls.Load())
	}
	snapshots.mu.Lock()
	if len(snapshots.entries) != 1 {
		t.Errorf("refresh changed snapshot key: entries=%d", len(snapshots.entries))
	}
	snapshots.mu.Unlock()
	// Expire only the representation; the hour-long document cache must now hold
	// the refreshed body, not resurrect the initial page.
	snapshots.mu.Lock()
	for _, entry := range snapshots.entries {
		entry.value.FetchedAt = time.Now().Add(-2 * time.Hour)
	}
	snapshots.mu.Unlock()
	after, err := snapshots.Representation(ctx, "source", "rss")
	if err != nil || after.Body != "updated" || calls.Load() != 2 {
		t.Fatalf("after snapshot expiry: body=%q calls=%d err=%v", after.Body, calls.Load(), err)
	}
}

func TestConfigurationPanicDoesNotLeakPreparationAdmission(t *testing.T) {
	var count atomic.Int64
	d := Dependencies{
		Lookup: func(string) (define.RuleCache, bool) { return define.RuleCache{}, true },
		Configure: func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
			if count.Add(1) <= 2 {
				panic("configuration panic")
			}
			return define.JavaScriptConfig{}, nil
		},
		Fetch: func(context.Context, define.JavaScriptConfig) define.BodyParsed { return define.BodyParsed{} },
		Serialize: func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
			return "ok", nil
		},
	}
	snapshots, err := NewSnapshots(t.Context(), New(40*time.Millisecond, d), SnapshotOptions{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshots.Close()
	ctx := access.WithScope(t.Context(), "alice\x00source", 1)
	for range 2 {
		func() { defer func() { _ = recover() }(); _, _ = snapshots.Representation(ctx, "source", "rss") }()
	}
	if _, err := snapshots.Representation(ctx, "source", "rss"); err != nil {
		t.Fatalf("after two recovered configuration panics, a healthy request still fails: %v (configuration calls=%d)", err, count.Load())
	}
}

func TestRefreshWaitsForOrdinaryFlightThenFetchesFresh(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int64
	d := Dependencies{
		Lookup: func(string) (define.RuleCache, bool) { return define.RuleCache{}, true },
		Configure: func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
			return define.JavaScriptConfig{}, nil
		},
		Fetch: func(ctx context.Context, _ define.JavaScriptConfig) define.BodyParsed {
			if calls.Add(1) == 1 {
				close(started)
				<-release
			}
			title := "ordinary"
			if network.CacheRefresh(ctx) {
				title = "fresh"
			}
			return define.BodyParsed{Body: []define.InfoItem{{Title: title}}}
		},
		Serialize: func(_ context.Context, _ define.JavaScriptConfig, d define.BodyParsed, _ string) (string, error) {
			return d.Body[0].Title, nil
		},
	}
	s, err := NewSnapshots(t.Context(), New(time.Second, d), SnapshotOptions{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ordinary := make(chan error, 1)
	go func() { _, err := s.Representation(t.Context(), "source", "rss"); ordinary <- err }()
	<-started
	refreshed := make(chan Representation, 1)
	failed := make(chan error, 1)
	go func() { v, err := s.Refresh(t.Context(), "source", "rss"); refreshed <- v; failed <- err }()
	// Wait until the refresh has joined the existing flight.
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		waiters := 0
		for _, e := range s.entries {
			waiters = e.waiters
		}
		s.mu.Unlock()
		if waiters == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not join flight")
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	if err := <-ordinary; err != nil {
		t.Fatal(err)
	}
	if v, err := <-refreshed, <-failed; err != nil || v.Body != "fresh" || calls.Load() != 2 {
		t.Fatalf("value=%+v err=%v calls=%d", v, err, calls.Load())
	}
}

func TestLookupPanicDoesNotLeakPreparationAdmission(t *testing.T) {
	var calls atomic.Int64
	d := Dependencies{
		Lookup: func(string) (define.RuleCache, bool) {
			if calls.Add(1) <= 2 {
				panic("lookup")
			}
			return define.RuleCache{}, true
		},
		Configure: func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
			return define.JavaScriptConfig{}, nil
		},
		Fetch: func(context.Context, define.JavaScriptConfig) define.BodyParsed { return define.BodyParsed{} },
		Serialize: func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
			return "ok", nil
		},
	}
	s, err := NewSnapshots(t.Context(), New(time.Second, d), SnapshotOptions{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := access.WithScope(t.Context(), "alice\x00source", 1)
	for range 2 {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("expected panic")
				}
			}()
			_, _ = s.Representation(ctx, "source", "rss")
		}()
	}
	if _, err := s.Representation(ctx, "source", "rss"); err != nil {
		t.Fatal(err)
	}
}

func TestManagedRevisionReachesFetchWithAndWithoutSnapshots(t *testing.T) {
	for _, useSnapshots := range []bool{false, true} {
		t.Run(map[bool]string{false: "service", true: "snapshots"}[useSnapshots], func(t *testing.T) {
			revision := []byte("revision-one")
			documents := map[string]string{}
			calls := 0
			d := Dependencies{
				Lookup: func(string) (define.RuleCache, bool) { return define.RuleCache{Managed: true, Body: revision}, true },
				Configure: func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
					return define.JavaScriptConfig{}, nil
				},
				Fetch: func(ctx context.Context, _ define.JavaScriptConfig) define.BodyParsed {
					key := network.DocumentCacheKey(ctx, "same-url")
					if key == "same-url" {
						t.Error("managed revision missing")
					}
					body, ok := documents[key]
					if !ok {
						calls++
						body = string(revision)
						documents[key] = body
					}
					return define.BodyParsed{Body: []define.InfoItem{{Title: body}}}
				},
				Serialize: func(_ context.Context, _ define.JavaScriptConfig, d define.BodyParsed, _ string) (string, error) {
					return d.Body[0].Title, nil
				},
			}
			svc := New(time.Second, d)
			generate := svc.Generate
			if useSnapshots {
				s, err := NewSnapshots(t.Context(), svc, SnapshotOptions{TTL: time.Minute})
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				generate = s.Generate
			}
			if body, err := generate(t.Context(), "source", "rss"); err != nil || body != "revision-one" {
				t.Fatalf("body=%q err=%v", body, err)
			}
			revision = []byte("revision-two")
			if body, err := generate(t.Context(), "source", "rss"); err != nil || body != "revision-two" {
				t.Fatalf("body=%q err=%v", body, err)
			}
			if calls != 2 {
				t.Fatalf("fetches=%d", calls)
			}
		})
	}
}
