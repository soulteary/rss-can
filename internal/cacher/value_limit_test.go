package cacher

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetWithinLimitInvalidatesOnlyScopedOversizeEntry(t *testing.T) {
	first, second := New(Config{MemoryEnabled: true}), New(Config{MemoryEnabled: true})
	defer first.Close()
	defer second.Close()
	a, b := WithCache(t.Context(), first), WithCache(t.Context(), second)
	for _, ctx := range []context.Context{a, b} {
		if err := SetWithTTL(ctx, "key", "oversize", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if value, found, err := GetWithinLimit(a, "key", 3); err != nil || found || value != "" {
		t.Fatalf("invalid accepted: %q %v %v", value, found, err)
	}
	if _, found, _ := GetWithStatus(a, "key"); found {
		t.Fatal("invalid entry remained")
	}
	if value, found, err := GetWithinLimit(b, "key", 8); err != nil || !found || value != "oversize" {
		t.Fatalf("other owner changed: %q %v %v", value, found, err)
	}
	if value, found, err := GetWithinLimit(a, "missing", 3); value != "" || found || err != nil {
		t.Fatalf("missing: %q %v %v", value, found, err)
	}
}
func TestGetWithinLimitPreservesCanceledCaller(t *testing.T) {
	cache := New(Config{MemoryEnabled: true})
	defer cache.Close()
	active := WithCache(t.Context(), cache)
	if err := SetWithTTL(active, "key", "oversize", time.Minute); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(active)
	cancel()
	if _, found, err := GetWithinLimit(ctx, "key", 3); found || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: %v %v", found, err)
	}
	if _, found, _ := GetWithStatus(active, "key"); !found {
		t.Fatal("canceled request mutated cache")
	}
}
