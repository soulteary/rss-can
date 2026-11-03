package feedservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
)

type snapshotFixture struct {
	mu     sync.Mutex
	config define.JavaScriptConfig
	body   string
	calls  atomic.Int64
	fetch  func(context.Context) define.BodyParsed
	clock  atomic.Int64
}

func newSnapshotFixture(t testing.TB, options SnapshotOptions) (*Snapshots, *snapshotFixture) {
	t.Helper()
	f := &snapshotFixture{config: define.JavaScriptConfig{Name: "fixture", URL: "https://example.org"}, body: "first"}
	f.clock.Store(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano())
	d := Dependencies{
		Lookup: func(id string) (define.RuleCache, bool) {
			return define.RuleCache{Body: []byte("rule:" + id)}, id != "missing"
		},
		Configure: func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.config, nil
		},
		Fetch: func(ctx context.Context, _ define.JavaScriptConfig) define.BodyParsed {
			f.calls.Add(1)
			f.mu.Lock()
			fetch, body := f.fetch, f.body
			f.mu.Unlock()
			if fetch != nil {
				return fetch(ctx)
			}
			return define.BodyParsed{Body: []define.InfoItem{{ID: "1", Title: body, Link: "https://example.org/1"}}}
		},
		Serialize: generator.GenerateFeedsByTypeContext,
	}
	if options.TTL == 0 {
		options.TTL = time.Minute
	}
	s, err := NewSnapshots(context.Background(), New(time.Second, d), options)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Unix(0, f.clock.Load()) }
	t.Cleanup(func() { _ = s.Close() })
	return s, f
}
func (f *snapshotFixture) advance(d time.Duration) { f.clock.Add(int64(d)) }
func (f *snapshotFixture) setFetch(fetch func(context.Context) define.BodyParsed) {
	f.mu.Lock()
	f.fetch = fetch
	f.mu.Unlock()
}
func snapshotGet(t testing.TB, s *Snapshots, ctx context.Context, id string) Representation {
	t.Helper()
	v, err := s.Representation(ctx, id, "rss")
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func waitRefresh(t testing.TB, s *Snapshots) {
	t.Helper()
	done := make(chan struct{})
	go func() { s.work.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not finish")
	}
}

func TestSnapshotsSeparateScopeVersionConfigurationAndFormat(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{})
	a := access.WithScope(t.Context(), "alice\x00source", 1)
	first := snapshotGet(t, s, a, "source")
	if first.ETag == "" || first.FetchedAt.IsZero() || first.ChangedAt.IsZero() {
		t.Fatal("missing metadata")
	}
	if again := snapshotGet(t, s, a, "source"); again.ETag != first.ETag || f.calls.Load() != 1 {
		t.Fatal("warm snapshot regenerated")
	}
	snapshotGet(t, s, access.WithScope(t.Context(), "bob\x00source", 1), "source")
	snapshotGet(t, s, access.WithScope(t.Context(), "alice\x00source", 2), "source")
	f.mu.Lock()
	f.config.Cookies = "changed-private-credential"
	f.mu.Unlock()
	snapshotGet(t, s, a, "source")
	if _, err := s.Representation(a, "source", "json"); err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 5 {
		t.Fatalf("scope/config/format mixed: %d", f.calls.Load())
	}
	if _, err := s.Representation(a, "missing", "rss"); err == nil {
		t.Fatal("missing rule hit old cache")
	}
	if _, err := s.Representation(a, "source", "invalid"); err == nil {
		t.Fatal("invalid format accepted")
	}
}

func TestSnapshotsDisableCacheNeverRetainsRepresentation(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{})
	ctx := access.WithScope(t.Context(), "alice\x00source", 1)
	snapshotGet(t, s, ctx, "source")
	f.mu.Lock()
	f.config.DisableCache = true
	f.body = "new"
	f.mu.Unlock()
	for range 2 {
		v := snapshotGet(t, s, ctx, "source")
		if v.ETag != "" || !strings.Contains(v.Body, "new") {
			t.Fatal("DisableCache reused snapshot")
		}
	}
	if f.calls.Load() != 3 {
		t.Fatalf("fetch calls = %d", f.calls.Load())
	}
}

func TestSnapshotsHugeRuleExpiryCannotOverflowFreshness(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{})
	f.mu.Lock()
	f.config.Expire = int(^uint(0) >> 1)
	f.mu.Unlock()
	snapshotGet(t, s, t.Context(), "source")
	snapshotGet(t, s, t.Context(), "source")
	if f.calls.Load() != 1 {
		t.Fatal("huge expiry caused repeated fetches")
	}
}

func TestSnapshotsColdCallsShareAndCancelIndependently(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{})
	started, release := make(chan struct{}), make(chan struct{})
	f.setFetch(func(ctx context.Context) define.BodyParsed {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return define.BodyParsed{Code: define.ERROR_CODE_NETWORK, Status: ctx.Err().Error()}
		}
		return define.BodyParsed{Body: []define.InfoItem{{Title: "shared"}}}
	})
	ctx, cancel := context.WithCancel(access.WithScope(t.Context(), "alice\x00source", 1))
	one := make(chan error, 1)
	go func() { _, err := s.Representation(ctx, "source", "rss"); one <- err }()
	<-started
	two := make(chan error, 1)
	go func() {
		_, err := s.Representation(access.WithScope(t.Context(), "alice\x00source", 1), "source", "rss")
		two <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		n := 0
		for _, e := range s.entries {
			n = e.waiters
		}
		s.mu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second waiter not registered")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-one; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	close(release)
	if err := <-two; err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 1 {
		t.Fatalf("calls=%d", f.calls.Load())
	}
}

func TestSnapshotsLastWaiterCancellationStopsWork(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{})
	started, stopped := make(chan struct{}), make(chan struct{})
	f.setFetch(func(ctx context.Context) define.BodyParsed {
		close(started)
		<-ctx.Done()
		close(stopped)
		return define.BodyParsed{Code: define.ERROR_CODE_NETWORK, Status: "canceled"}
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := s.Representation(ctx, "source", "rss"); done <- err }()
	<-started
	cancel()
	<-done
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("orphan work not canceled")
	}
	waitRefresh(t, s)
}

func TestSnapshotsStaleRefreshStableMetadataAndFailureWindow(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{TTL: time.Minute, MaxStale: time.Hour, RetryInterval: time.Minute})
	first := snapshotGet(t, s, t.Context(), "source")
	f.advance(time.Minute)
	stale := snapshotGet(t, s, t.Context(), "source")
	if !stale.Stale || stale.ETag != first.ETag {
		t.Fatal("stale snapshot not returned")
	}
	waitRefresh(t, s)
	unchanged := snapshotGet(t, s, t.Context(), "source")
	if unchanged.Stale || unchanged.ETag != first.ETag || !unchanged.ChangedAt.Equal(first.ChangedAt) || !unchanged.FetchedAt.After(first.FetchedAt) {
		t.Fatalf("unchanged metadata %+v", unchanged)
	}
	f.mu.Lock()
	f.body = "changed"
	f.mu.Unlock()
	f.advance(time.Minute)
	snapshotGet(t, s, t.Context(), "source")
	waitRefresh(t, s)
	changed := snapshotGet(t, s, t.Context(), "source")
	if changed.ETag == first.ETag || !changed.ChangedAt.After(first.ChangedAt) {
		t.Fatal("changed snapshot metadata retained")
	}
	f.setFetch(func(context.Context) define.BodyParsed {
		return define.BodyParsed{Code: define.ERROR_CODE_NETWORK, Status: "unavailable"}
	})
	f.advance(time.Minute)
	snapshotGet(t, s, t.Context(), "source")
	waitRefresh(t, s)
	calls := f.calls.Load()
	again := snapshotGet(t, s, t.Context(), "source")
	if !again.Stale || again.ETag != changed.ETag || f.calls.Load() != calls {
		t.Fatal("failed refresh discarded value or ignored backoff")
	}
	f.advance(2 * time.Hour)
	if _, err := s.Representation(t.Context(), "source", "rss"); err == nil {
		t.Fatal("expired stale value served")
	}
}

func TestSnapshotsCapacityBoundsAndClose(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{MaxEntries: 1, MaxBytes: 4096, TTL: time.Minute, MaxStale: time.Hour})
	snapshotGet(t, s, t.Context(), "one")
	snapshotGet(t, s, t.Context(), "two")
	s.mu.Lock()
	entries, bytes := len(s.entries), s.bytes
	s.mu.Unlock()
	if entries != 1 || bytes > 4096 {
		t.Fatal("capacity exceeded")
	}
	started, stopped := make(chan struct{}), make(chan struct{})
	f.setFetch(func(ctx context.Context) define.BodyParsed {
		close(started)
		<-ctx.Done()
		close(stopped)
		return define.BodyParsed{Code: define.ERROR_CODE_NETWORK}
	})
	f.advance(time.Minute)
	snapshotGet(t, s, t.Context(), "two")
	<-started
	if _, err := s.Representation(t.Context(), "three", "rss"); err == nil {
		t.Fatal("running entry evicted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("close returned before refresh stopped")
	}
	if _, err := s.Representation(t.Context(), "two", "rss"); err == nil {
		t.Fatal("closed service accepted request")
	}
}

func TestSnapshotsRejectOversizeAndBadOptions(t *testing.T) {
	s, _ := newSnapshotFixture(t, SnapshotOptions{MaxBytes: 1})
	if _, err := s.Representation(t.Context(), "source", "rss"); err == nil {
		t.Fatal("oversized snapshot stored")
	}
	for _, o := range []SnapshotOptions{{}, {TTL: -1}, {TTL: 1, MaxBytes: -1}, {TTL: 1, MaxEntries: -1}, {TTL: 1, MaxStale: -1}, {TTL: 1, RetryInterval: -1}, {TTL: time.Hour, MaxStale: time.Duration(1<<63 - 1)}} {
		if v, err := NewSnapshots(t.Context(), s.service, o); err == nil {
			v.Close()
			t.Fatalf("invalid options accepted %+v", o)
		}
	}
}

func TestSnapshotsRefreshIsScopedAndBypassesFreshness(t *testing.T) {
	s, f := newSnapshotFixture(t, SnapshotOptions{})
	ctx := access.WithScope(t.Context(), "alice\x00source", 1)
	before := snapshotGet(t, s, ctx, "source")
	f.mu.Lock()
	f.body = "refreshed"
	f.mu.Unlock()
	f.advance(time.Second)
	after, err := s.Refresh(ctx, "source", "rss")
	if err != nil || after.ETag == before.ETag || f.calls.Load() != 2 {
		t.Fatalf("refresh=%+v err=%v calls=%d", after, err, f.calls.Load())
	}
}

func TestSnapshotsRecoverDependencyPanicAndReleaseWaiters(t *testing.T) {
	for _, stage := range []string{"fetch", "serialize"} {
		t.Run(stage, func(t *testing.T) {
			s, f := newSnapshotFixture(t, SnapshotOptions{})
			serialize := s.service.dependencies.Serialize
			if stage == "fetch" {
				f.setFetch(func(context.Context) define.BodyParsed { panic("must-not-leak-private-source") })
			} else {
				s.service.dependencies.Serialize = func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
					panic("must-not-leak-private-source")
				}
			}
			_, err := s.Representation(t.Context(), "source", "rss")
			if err == nil || strings.Contains(err.Error(), "must-not-leak") {
				t.Fatalf("unsafe panic recovery: %v", err)
			}
			waitRefresh(t, s)
			s.fair.mu.Lock()
			active, queued := s.fair.active, s.fair.queued
			s.fair.mu.Unlock()
			if active != 0 || queued != 0 {
				t.Fatal("panic leaked execution slots")
			}
			f.setFetch(nil)
			s.service.dependencies.Serialize = serialize
			f.advance(time.Minute)
			if v := snapshotGet(t, s, t.Context(), "source"); v.Body == "" {
				t.Fatal("worker did not recover after panic")
			}
		})
	}
}

func TestSnapshotsErrorBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Snapshots, *snapshotFixture)
		stage  Stage
	}{
		{"configuration", func(s *Snapshots, _ *snapshotFixture) {
			s.service.dependencies.Configure = func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
				return define.JavaScriptConfig{}, errors.New("bad configuration")
			}
		}, Configuration},
		{"fetch content limit", func(_ *Snapshots, f *snapshotFixture) {
			f.setFetch(func(context.Context) define.BodyParsed { return define.BodyParsed{Body: make([]define.InfoItem, 1001)} })
		}, Fetch},
		{"serializer", func(s *Snapshots, _ *snapshotFixture) {
			s.service.dependencies.Serialize = func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
				return "", errors.New("serialization failed")
			}
		}, Serialization},
		{"serialized size", func(s *Snapshots, _ *snapshotFixture) {
			s.service.dependencies.Serialize = func(context.Context, define.JavaScriptConfig, define.BodyParsed, string) (string, error) {
				return strings.Repeat("x", (10<<20)+1), nil
			}
		}, Serialization},
		{"capacity", func(s *Snapshots, _ *snapshotFixture) {
			s.fair = newFairAdmission(1, 0, 1, 1)
			_, _ = s.fair.acquire(context.Background(), "blocking")
		}, Admission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := newSnapshotFixture(t, SnapshotOptions{})
			tc.change(s, f)
			_, err := s.Representation(t.Context(), "source", "rss")
			var failure *Error
			if !errors.As(err, &failure) || failure.Stage != tc.stage {
				t.Fatalf("%v", err)
			}
		})
	}
	s, _ := newSnapshotFixture(t, SnapshotOptions{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.Representation(ctx, "source", "rss"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func BenchmarkSnapshotWarm(b *testing.B) {
	s, _ := newSnapshotFixture(b, SnapshotOptions{})
	ctx := access.WithScope(b.Context(), "alice\x00source", 1)
	snapshotGet(b, s, ctx, "source")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.Representation(ctx, "source", "rss"); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkSnapshotCold(b *testing.B) {
	s, _ := newSnapshotFixture(b, SnapshotOptions{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		if _, err := s.Representation(b.Context(), fmt.Sprint(i), "rss"); err != nil {
			b.Fatal(err)
		}
	}
}
