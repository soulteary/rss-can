package cacher

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/cache-kit/v2/ttlcache"
)

var noAlivedBackend = (func() error {
	return errors.New("no cache enabled")
})()

var instanceMemory *ttlcache.Memory[string, []byte]
var instanceRedis *redis.Client
var instanceService *Service

var initializeOnce sync.Once

// Initialize creates clients after command-line and environment configuration is applied.
// Redis connects lazily on its first command. Configuration is fixed for this process.
func Initialize() {
	initializeOnce.Do(func() {
		instanceMemory = InitializeMemory(true, define.DEFAULT_IN_MEMORY_CACHE_STORE_NAME)
		instanceRedis = InitializeRedis(true)
		instanceService = NewService(
			newRedisBackend(instanceRedis),
			newMemoryBackend(instanceMemory),
			func() bool { return define.REDIS },
			func() bool { return define.IN_MEMORY_CACHE },
		)
	})
}

func Get(key string) (string, error) {
	value, _, err := GetWithStatus(context.Background(), key)
	return value, err
}

func GetWithStatus(ctx context.Context, key string) (string, bool, error) {
	if cache := scopedCache(ctx); cache != nil {
		return cache.service.Get(ctx, key)
	}
	Initialize()
	return instanceService.Get(ctx, key)
}

func FallbackToMemoryGet(key string) (string, error) {
	Initialize()
	data, err := GetDataFromMemory(instanceMemory, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Set(key string, value string) error {
	return SetWithTTL(context.Background(), key, value, 0)
}

func SetWithTTL(ctx context.Context, key, value string, ttl time.Duration) error {
	if cache := scopedCache(ctx); cache != nil {
		return cache.service.Set(ctx, key, value, ttl)
	}
	Initialize()
	return instanceService.Set(ctx, key, value, ttl)
}

func FallbackToMemorySet(key string, value string) error {
	Initialize()
	UpdateDataToMemory(instanceMemory, key, value)
	return nil
}

func Del(key string) error {
	return DeleteWithContext(context.Background(), key)
}

// DeleteWithContext invalidates only the cache selected by the caller's scope.
func DeleteWithContext(ctx context.Context, key string) error {
	if cache := scopedCache(ctx); cache != nil {
		return cache.service.Delete(ctx, key)
	}
	Initialize()
	return instanceService.Delete(ctx, key)
}

// GetWithinLimit treats oversized values as invalid cache entries. Deletion is
// best effort because a failed cache must not stop a fresh bounded fetch. The
// backend may allocate the value while reading; this bounds what reaches parsers.
func GetWithinLimit(ctx context.Context, key string, maxBytes int) (string, bool, error) {
	value, found, err := GetWithStatus(ctx, key)
	if canceled := ctx.Err(); canceled != nil {
		return "", false, canceled
	}
	if err != nil || !found {
		return "", false, err
	}
	if len(value) <= maxBytes {
		return value, true, nil
	}
	_ = DeleteWithContext(ctx, key)
	if canceled := ctx.Err(); canceled != nil {
		return "", false, canceled
	}
	return "", false, nil
}

func FallbackToMemoryDel(key string) error {
	Initialize()
	DelDataByKeyFromMemory(instanceMemory, key)
	return nil
}

func Expire(key string, expire time.Duration) error {
	Initialize()
	if define.REDIS {
		err := instanceService.primaryCall(context.Background(), func(ctx context.Context) error {
			return setDataExpireByKeyFromRedisContext(ctx, instanceRedis, key, expire)
		})
		if err != nil {
			if define.IN_MEMORY_CACHE {
				return FallbackToMemoryExpire(key, expire)
			}
			return err
		}
		return nil
	}

	if define.IN_MEMORY_CACHE {
		return FallbackToMemoryExpire(key, expire)
	}
	return noAlivedBackend
}

func FallbackToMemoryExpire(key string, expire time.Duration) error {
	Initialize()
	err := SetDataExpireByKeyFromMemory(instanceMemory, key, expire)
	if err != nil {
		return err
	}
	return nil
}

func IsEnable() bool {
	Initialize()
	return instanceService.Enabled()
}

// Close releases the cache backends initialized for this process.
func Close() error {
	Initialize()
	return instanceService.Close()
}
