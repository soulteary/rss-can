package cacher

import (
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/muesli/cache2go"
	"github.com/soulteary/RSS-Can/internal/define"
)

var noAlivedBackend = (func() error {
	return errors.New("no cache enabled")
})()

var instanceMemory *cache2go.CacheTable
var instanceRedis *redis.Client

func init() {
	instanceMemory = InitializeMemory(define.IN_MEMORY_CACHE, define.DEFAULT_IN_MEMORY_CACHE_STORE_NAME)
	instanceRedis = InitializeRedis(define.REDIS)
}

func Get(key string) (string, error) {
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
	data, err := GetDataFromMemory(instanceMemory, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Set(key string, value string) error {
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
	UpdateDataToMemory(instanceMemory, key, value)
	return nil
}

func Del(key string) error {
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
	DelDataByKeyFromMemory(instanceMemory, key)
	return nil
}

func Expire(key string, expire time.Duration) error {
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
	err := SetDataExpireByKeyFromMemory(instanceMemory, key, expire)
	if err != nil {
		return err
	}
	return nil
}

func IsEnable() bool {
	return define.IN_MEMORY_CACHE || define.REDIS
}
