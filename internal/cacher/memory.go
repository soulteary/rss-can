package cacher

import (
	"context"
	"errors"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/telemetry"
	"github.com/soulteary/cache-kit/v2/ttlcache"
)

type memoryBackend struct {
	cache *ttlcache.Memory[string, []byte]
}

func newMemoryBackend(cache *ttlcache.Memory[string, []byte]) *memoryBackend {
	return &memoryBackend{cache: cache}
}

func (b *memoryBackend) Get(_ context.Context, key string) (string, bool, error) {
	value, found := b.cache.Get(key)
	return string(value), found, nil
}

func (b *memoryBackend) Set(_ context.Context, key, value string, ttl time.Duration) error {
	return setMemory(b.cache, key, value, ttl)
}

func (b *memoryBackend) Delete(_ context.Context, key string) error {
	b.cache.Delete(key)
	return nil
}

func (b *memoryBackend) Close() error {
	return b.cache.Close()
}

// Memory capacity accounts for retained key/value bytes, not Go allocator or
// map overhead. The entry limit additionally bounds bookkeeping allocations.
const (
	MemoryMaxBytes   int64 = 64 << 20
	MemoryMaxEntries       = 1024
	// Accommodate a maximum-size SSR document and its hashed cache key.
	MemoryMaxItemBytes int64 = (10 << 20) + 1024
)

func memoryConfig(ttl time.Duration, maxEntries int, maxBytes, maxItemBytes int64) ttlcache.Config[string, []byte] {
	return ttlcache.Config[string, []byte]{
		DefaultTTL: ttl, MaxEntries: maxEntries, MaxCost: maxBytes, MaxItemCost: maxItemBytes,
		Cost: func(key string, value []byte) int64 { return int64(len(key)) + int64(len(value)) },
		OnEvict: func(_ string, _ []byte, reason ttlcache.EvictionReason) {
			if reason == ttlcache.EvictedCapacity {
				telemetry.Default.Add("cache_memory_evictions_total", 1)
			}
		},
	}
}

func setMemory(cache *ttlcache.Memory[string, []byte], key, value string, ttl time.Duration) error {
	err := cache.Set(key, []byte(value), ttl)
	if errors.Is(err, ttlcache.ErrItemTooLarge) {
		telemetry.Default.Add("cache_memory_rejections_total", 1)
	}
	return err
}

func newMemory(ttl time.Duration) *ttlcache.Memory[string, []byte] {
	return ttlcache.New(memoryConfig(ttl, MemoryMaxEntries, MemoryMaxBytes, MemoryMaxItemBytes))
}

func InitializeMemory(enabled bool, _ string) *ttlcache.Memory[string, []byte] {
	if enabled {
		return newMemory(fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION))
	}
	return nil
}

func IsMemoryEmpty(instance *ttlcache.Memory[string, []byte]) bool {
	return instance.Len() == 0
}

func UpdateDataToMemory(instance *ttlcache.Memory[string, []byte], key, value string) {
	_ = setMemory(instance, key, value, 0)
}

func GetDataFromMemory(instance *ttlcache.Memory[string, []byte], key string) ([]byte, error) {
	value, ok := instance.Get(key)
	if !ok {
		return []byte(""), nil
	}
	return value, nil
}

func DelDataByKeyFromMemory(instance *ttlcache.Memory[string, []byte], key string) {
	instance.Delete(key)
}

func SetDataExpireByKeyFromMemory(instance *ttlcache.Memory[string, []byte], key string, expire time.Duration) error {
	instance.Expire(key, expire)
	return nil
}

func FlushDataFromMemory(instance *ttlcache.Memory[string, []byte]) {
	instance.Clear()
}
