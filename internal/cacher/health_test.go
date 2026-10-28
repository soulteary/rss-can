package cacher

import (
	"context"
	"errors"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

func TestReadinessCachePolicy(t *testing.T) {
	for _, tc := range []struct {
		redis, memory, fail bool
		want                Status
	}{
		{true, true, false, Status{Ready: true, Backend: "redis"}},
		{true, true, true, Status{Ready: true, Backend: "memory", Degraded: true}},
		{true, false, true, Status{Backend: "redis", Degraded: true}},
		{false, true, false, Status{Ready: true, Backend: "memory"}},
		{false, false, false, Status{Ready: true, Backend: "disabled"}},
	} {
		calls := 0
		got := check(context.Background(), tc.redis, tc.memory, func(context.Context) error {
			calls++
			if tc.fail {
				return errors.New("offline")
			}
			return nil
		})
		if got != tc.want {
			t.Fatalf("%+v => %+v", tc, got)
		}
		if !tc.redis && calls != 0 {
			t.Fatal("disabled Redis was probed")
		}
	}
}
func TestServiceReadMetrics(t *testing.T) {
	primary := &stubBackend{value: "cached", found: true}
	fallback := &stubBackend{value: "backup", found: true}
	service := NewService(primary, fallback, func() bool { return true }, func() bool { return true })
	hits := telemetry.Default.Value("cache_hits_total")
	misses := telemetry.Default.Value("cache_misses_total")
	fallbacks := telemetry.Default.Value("cache_fallbacks_total")
	failures := telemetry.Default.Value("cache_errors_total")
	service.Get(context.Background(), "key")
	primary.found = false
	service.Get(context.Background(), "key")
	primary.err = errors.New("offline")
	service.Get(context.Background(), "key")
	if telemetry.Default.Value("cache_hits_total") != hits+2 || telemetry.Default.Value("cache_misses_total") != misses+1 || telemetry.Default.Value("cache_fallbacks_total") != fallbacks+1 || telemetry.Default.Value("cache_errors_total") != failures+1 {
		t.Fatal("incorrect cache event accounting")
	}
}
func TestServiceWriteDeleteFallbackAndDisabled(t *testing.T) {
	primary := &stubBackend{err: errors.New("offline")}
	backup := &stubBackend{}
	enabled := true
	fallback := true
	s := NewService(primary, backup, func() bool { return enabled }, func() bool { return fallback })
	if s.Set(context.Background(), "key", "value", 0) != nil || backup.sets != 1 {
		t.Fatal("write fallback failed")
	}
	if s.Delete(context.Background(), "key") != nil || backup.deletes != 1 {
		t.Fatal("delete fallback failed")
	}
	fallback = false
	if s.Set(context.Background(), "key", "value", 0) == nil || s.Delete(context.Background(), "key") == nil {
		t.Fatal("primary error lost")
	}
	enabled = false
	if s.Set(context.Background(), "key", "value", 0) == nil || s.Delete(context.Background(), "key") == nil {
		t.Fatal("disabled backends succeeded")
	}
	if s.Set(context.Background(), "key", "value", -1) == nil {
		t.Fatal("negative ttl accepted")
	}
}

func TestCheckRuntimeConfiguration(t *testing.T) {
	redis, memory := define.REDIS, define.IN_MEMORY_CACHE
	defer func() { define.REDIS, define.IN_MEMORY_CACHE = redis, memory }()
	for _, tc := range []struct {
		redis, memory bool
		backend       string
	}{{false, true, "memory"}, {false, false, "disabled"}, {true, true, "redis"}} {
		define.REDIS, define.IN_MEMORY_CACHE = tc.redis, tc.memory
		got := Check(context.Background())
		if !got.Ready {
			t.Fatalf("%+v", got)
		}
		if !tc.redis && got.Backend != tc.backend {
			t.Fatalf("%+v", got)
		}
	}
}
