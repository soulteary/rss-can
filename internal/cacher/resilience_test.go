package cacher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type resilienceBackend struct {
	get   func(context.Context) (string, bool, error)
	calls atomic.Int32
}

func (b *resilienceBackend) Get(ctx context.Context, _ string) (string, bool, error) {
	b.calls.Add(1)
	return b.get(ctx)
}
func (b *resilienceBackend) Set(ctx context.Context, _, _ string, _ time.Duration) error {
	_, _, err := b.Get(ctx, "")
	return err
}
func (b *resilienceBackend) Delete(ctx context.Context, _ string) error {
	_, _, err := b.Get(ctx, "")
	return err
}
func (b *resilienceBackend) Close() error { return nil }

func TestPrimaryBudgetAndOpenCircuitBypass(t *testing.T) {
	primary := &resilienceBackend{get: func(ctx context.Context) (string, bool, error) { <-ctx.Done(); return "", false, ctx.Err() }}
	fallback := &stubBackend{value: "backup", found: true}
	s := NewService(primary, fallback, func() bool { return true }, func() bool { return true })
	s.primaryTimeout = 10 * time.Millisecond
	for range 2 {
		value, found, err := s.Get(t.Context(), "key")
		if err != nil || !found || value != "backup" {
			t.Fatalf("fallback: %q %t %v", value, found, err)
		}
	}
	if primary.calls.Load() != 1 {
		t.Fatalf("open circuit attempted Redis %d times", primary.calls.Load())
	}
}

func TestHalfOpenAllowsSingleProbeAndRecovers(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var healthy atomic.Bool
	primary := &resilienceBackend{get: func(ctx context.Context) (string, bool, error) {
		if !healthy.Load() {
			return "", false, errors.New("offline")
		}
		close(entered)
		select {
		case <-release:
			return "redis", true, nil
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
	}}
	s := NewService(primary, &stubBackend{value: "backup", found: true}, func() bool { return true }, func() bool { return true })
	now := time.Now()
	s.now = func() time.Time { return now }
	s.Get(t.Context(), "key")
	now = now.Add(PrimaryRetryInterval)
	healthy.Store(true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		value, _, err := s.Get(t.Context(), "key")
		if value != "redis" || err != nil {
			t.Errorf("probe: %q %v", value, err)
		}
	}()
	<-entered
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			value, _, err := s.Get(t.Context(), "key")
			if value != "backup" || err != nil {
				t.Errorf("parallel fallback: %q %v", value, err)
			}
		})
	}
	wg.Wait()
	if primary.calls.Load() != 2 {
		t.Fatalf("concurrent probes: %d", primary.calls.Load())
	}
	close(release)
	<-done
	s.breakerMu.Lock()
	open := !s.openUntil.IsZero()
	s.breakerMu.Unlock()
	if open {
		t.Fatal("successful probe did not close circuit")
	}
	// A following request reaches Redis, not the outage cache.
	primary.get = func(context.Context) (string, bool, error) { return "recovered", true, nil }
	value, _, err := s.Get(t.Context(), "key")
	if value != "recovered" || err != nil {
		t.Fatalf("recovered request: %q %v", value, err)
	}
}

func TestCallerCancellationDoesNotTripCircuitOrUseFallback(t *testing.T) {
	primary := &resilienceBackend{get: func(ctx context.Context) (string, bool, error) { <-ctx.Done(); return "", false, ctx.Err() }}
	s := NewService(primary, &stubBackend{value: "backup", found: true}, func() bool { return true }, func() bool { return true })
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	if _, _, err := s.Get(ctx, "key"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller cancellation lost: %v", err)
	}
	if !s.openUntil.IsZero() {
		t.Fatal("caller deadline tripped primary circuit")
	}
	canceled, cancelNow := context.WithCancel(t.Context())
	cancelNow()
	if err := s.Set(canceled, "key", "v", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled set: %v", err)
	}
	if err := s.Delete(canceled, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled delete: %v", err)
	}
}

func TestSuccessfulPrimaryWritesInvalidateOutageValues(t *testing.T) {
	backup := newMemoryBackend(InitializeMemory(true, "test"))
	defer backup.Close()
	primary := &stubBackend{}
	s := NewService(primary, backup, func() bool { return true }, func() bool { return true })
	for _, operation := range []string{"set", "delete"} {
		backup.Set(t.Context(), "key", "stale", time.Minute)
		var err error
		if operation == "set" {
			err = s.Set(t.Context(), "key", "new", time.Minute)
		} else {
			err = s.Delete(t.Context(), "key")
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, found, _ := backup.Get(t.Context(), "key"); found {
			t.Fatalf("%s left stale fallback", operation)
		}
	}
}

func TestPrimaryWriteBudgetsWithoutFallback(t *testing.T) {
	for _, operation := range []string{"set", "delete"} {
		t.Run(operation, func(t *testing.T) {
			primary := &resilienceBackend{get: func(ctx context.Context) (string, bool, error) { <-ctx.Done(); return "", false, ctx.Err() }}
			s := NewService(primary, &stubBackend{}, func() bool { return true }, func() bool { return false })
			s.primaryTimeout = time.Millisecond
			var err error
			if operation == "set" {
				err = s.Set(t.Context(), "key", "value", time.Minute)
			} else {
				err = s.Delete(t.Context(), "key")
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("primary budget lost: %v", err)
			}
		})
	}
}

func TestFailedHalfOpenProbeStartsNewCooldown(t *testing.T) {
	primary := &resilienceBackend{get: func(context.Context) (string, bool, error) { return "", false, errors.New("offline") }}
	s := NewService(primary, &stubBackend{value: "backup", found: true}, func() bool { return true }, func() bool { return true })
	now := time.Now()
	s.now = func() time.Time { return now }
	s.Get(t.Context(), "key")
	now = now.Add(PrimaryRetryInterval)
	s.Get(t.Context(), "key")
	s.Get(t.Context(), "key")
	if primary.calls.Load() != 2 {
		t.Fatalf("failed probe did not reopen circuit: %d attempts", primary.calls.Load())
	}
}
