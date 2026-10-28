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
	Initialize()
	return instanceService.Set(ctx, key, value, ttl)
}

func FallbackToMemorySet(key string, value string) error {
	Initialize()
	UpdateDataToMemory(instanceMemory, key, value)
	return nil
}

func Del(key string) error {
	Initialize()
	return instanceService.Delete(context.Background(), key)
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
