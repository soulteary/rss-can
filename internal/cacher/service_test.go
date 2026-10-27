package cacher

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubBackend struct {
	value   string
	found   bool
	err     error
	ttl     time.Duration
	sets    int
	deletes int
	closes  int
}

func (b *stubBackend) Get(context.Context, string) (string, bool, error) {
	return b.value, b.found, b.err
}

func (b *stubBackend) Set(_ context.Context, _, value string, ttl time.Duration) error {
	b.sets++
	b.value, b.found, b.ttl = value, true, ttl
	return b.err
}

func (b *stubBackend) Delete(context.Context, string) error {
	b.deletes++
	return b.err
}

func (b *stubBackend) Close() error {
	b.closes++
	return b.err
}

func TestServicePreservesEmptyCachedValueAndTTL(t *testing.T) {
	primary := &stubBackend{value: "", found: true}
	service := NewService(primary, &stubBackend{}, func() bool { return true }, func() bool { return false })

	value, found, err := service.Get(t.Context(), "empty")
	if err != nil || !found || value != "" {
		t.Fatalf("empty cached value: value=%q found=%t err=%v", value, found, err)
	}

	ttl := 3 * time.Minute
	if err := service.Set(t.Context(), "key", "value", ttl); err != nil {
		t.Fatal(err)
	}
	if primary.sets != 1 || primary.ttl != ttl {
		t.Fatalf("atomic TTL write: sets=%d ttl=%v", primary.sets, primary.ttl)
	}
}

func TestServiceFallsBackOnlyWhenPrimaryFails(t *testing.T) {
	primary := &stubBackend{err: errors.New("redis unavailable")}
	fallback := &stubBackend{value: "fallback", found: true}
	service := NewService(primary, fallback, func() bool { return true }, func() bool { return true })

	value, found, err := service.Get(t.Context(), "key")
	if err != nil || !found || value != "fallback" {
		t.Fatalf("fallback get: value=%q found=%t err=%v", value, found, err)
	}
	if err := service.Set(t.Context(), "key", "value", time.Minute); err != nil {
		t.Fatal(err)
	}
	if fallback.sets != 1 {
		t.Fatalf("fallback writes=%d, want 1", fallback.sets)
	}
}

func TestServiceClosesEveryBackend(t *testing.T) {
	primary := &stubBackend{err: errors.New("primary close")}
	fallback := &stubBackend{err: errors.New("fallback close")}
	service := NewService(primary, fallback, func() bool { return true }, func() bool { return true })

	err := service.Close()
	if !errors.Is(err, primary.err) || !errors.Is(err, fallback.err) {
		t.Fatalf("joined close error: %v", err)
	}
	if primary.closes != 1 || fallback.closes != 1 {
		t.Fatalf("close counts: primary=%d fallback=%d", primary.closes, fallback.closes)
	}
}
