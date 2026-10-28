package cacher

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/telemetry"
	"github.com/soulteary/cache-kit/v2/ttlcache"
)

func TestMemoryCapacityUsesBytesAndLRUEviction(t *testing.T) {
	cache := ttlcache.New(memoryConfig(time.Hour, 10, 10, 10))
	defer cache.Close()
	before := telemetry.Default.Value("cache_memory_evictions_total")
	for _, key := range []string{"a", "b"} {
		if err := setMemory(cache, key, "1234", 0); err != nil {
			t.Fatal(err)
		}
	}
	// Each entry costs five bytes; accessing a makes b the eviction victim.
	cache.Get("a")
	if err := setMemory(cache, "c", "1234", 0); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.Get("b"); found {
		t.Fatal("least recently used entry survived capacity eviction")
	}
	if _, found := cache.Get("a"); !found {
		t.Fatal("recent entry was evicted")
	}
	stats := cache.Stats()
	if stats.Cost != 10 || stats.Entries != 2 || stats.Evictions != 1 {
		t.Fatalf("capacity accounting: %+v", stats)
	}
	if telemetry.Default.Value("cache_memory_evictions_total") != before+1 {
		t.Fatal("capacity eviction was not counted")
	}
	cache.Delete("a")
	cache.Clear()
	if telemetry.Default.Value("cache_memory_evictions_total") != before+1 {
		t.Fatal("explicit deletion incorrectly counted as capacity eviction")
	}
}

func TestMemoryCapacityRejectsOversizeWithoutReplacingValue(t *testing.T) {
	cache := ttlcache.New(memoryConfig(time.Hour, 10, 20, 6))
	defer cache.Close()
	if err := setMemory(cache, "key", "old", 0); err != nil {
		t.Fatal(err)
	}
	before := telemetry.Default.Value("cache_memory_rejections_total")
	backend := newMemoryBackend(cache)
	if err := backend.Set(t.Context(), "key", "large", 0); !errors.Is(err, ttlcache.ErrItemTooLarge) {
		t.Fatalf("oversize: %v", err)
	}
	value, found, err := backend.Get(t.Context(), "key")
	if err != nil || !found || value != "old" {
		t.Fatalf("rejected update destroyed old value: %q %v %v", value, found, err)
	}
	if telemetry.Default.Value("cache_memory_rejections_total") != before+1 {
		t.Fatal("oversize rejection was not counted")
	}
	if stats := cache.Stats(); stats.Cost != 6 || stats.Rejections != 1 {
		t.Fatalf("rejection accounting: %+v", stats)
	}
}

func TestDefaultMemoryCapacityBoundsTTLWindow(t *testing.T) {
	cache := InitializeMemory(true, "")
	defer cache.Close()
	// Long-lived distinct keys cannot accumulate beyond the entry budget.
	for i := 0; i < MemoryMaxEntries+10; i++ {
		if err := setMemory(cache, fmt.Sprint(i), "v", time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if stats := cache.Stats(); stats.Entries != MemoryMaxEntries || stats.Cost > MemoryMaxBytes {
		t.Fatalf("default entry budget: %+v", stats)
	}
	// A few maximum-sized pages exceed total byte budget well before entry limit.
	cache.Clear()
	page := strings.Repeat("x", 10<<20)
	for i := 0; i < 10; i++ {
		if err := setMemory(cache, fmt.Sprint(i), page, time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if stats := cache.Stats(); stats.Cost > MemoryMaxBytes || stats.Evictions == 0 || stats.Entries >= 10 {
		t.Fatalf("default byte budget: %+v", stats)
	}
	if err := setMemory(cache, "oversize", strings.Repeat("x", int(MemoryMaxItemBytes)), 0); !errors.Is(err, ttlcache.ErrItemTooLarge) {
		t.Fatalf("default item budget: %v", err)
	}
}
