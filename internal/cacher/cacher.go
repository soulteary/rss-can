package cacher

import (
	"errors"
	"sync"
	"time"

	"github.com/muesli/cache2go"
	"github.com/redis/go-redis/v9"
	"github.com/soulteary/RSS-Can/internal/define"
)

var noAlivedBackend = (func() error {
	return errors.New("no cache enabled")
})()

var instanceMemory *cache2go.CacheTable
var instanceRedis *redis.Client

var initializeOnce sync.Once

// Initialize creates clients after command-line and environment configuration is applied.
// Redis connects lazily on its first command. Configuration is fixed for this process.
func Initialize() {
	initializeOnce.Do(func() {
		instanceMemory = InitializeMemory(true, define.DEFAULT_IN_MEMORY_CACHE_STORE_NAME)
		instanceRedis = InitializeRedis(true)
	})
}

func Get(key string) (string, error) {
	Initialize()
	if define.REDIS {
		str, err := GetDataFromRedis(instanceRedis, key)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				return FallbackToMemoryGet(key)
			}
			return "", err
		}
		return str, nil
	}

	if define.IN_MEMORY_CACHE {
		return FallbackToMemoryGet(key)
	}
	return "", noAlivedBackend
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
	Initialize()
	if define.REDIS {
		err := UpdateDataToRedis(instanceRedis, key, value)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				return FallbackToMemorySet(key, value)
			}
			return err
		}
		return nil
	}

	if define.IN_MEMORY_CACHE {
		return FallbackToMemorySet(key, value)
	}
	return noAlivedBackend
}

func FallbackToMemorySet(key string, value string) error {
	Initialize()
	UpdateDataToMemory(instanceMemory, key, value)
	return nil
}

func Del(key string) error {
	Initialize()
	if define.REDIS {
		err := DelDataByKeyFromRedis(instanceRedis, key)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				return FallbackToMemoryDel(key)
			}
			return err
		}
		return nil
	}

	if define.IN_MEMORY_CACHE {
		return FallbackToMemoryDel(key)
	}
	return noAlivedBackend
}

func FallbackToMemoryDel(key string) error {
	Initialize()
	DelDataByKeyFromMemory(instanceMemory, key)
	return nil
}

func Expire(key string, expire time.Duration) error {
	Initialize()
	if define.REDIS {
		err := SetDataExpireByKeyFromRedis(instanceRedis, key, expire)
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
	return define.IN_MEMORY_CACHE || define.REDIS
}
