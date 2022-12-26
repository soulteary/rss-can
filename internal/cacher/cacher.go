package cacher

import (
	"errors"
	"time"

	"github.com/muesli/cache2go"
	"github.com/soulteary/RSS-Can/internal/define"
)

var NO_CACHE_ENABLED = (func() error {
	return errors.New("no cache enabled")
})()

var instanceMemory *cache2go.CacheTable

func init() {
	instanceMemory = InitializeMemory(define.IN_MEMORY_CACHE, define.DEFAULT_IN_MEMORY_CACHE_STORE_NAME)
}

func Get(key string) (string, error) {
	if define.REDIS {
		str, err := GetDataFromRedis(key)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				data, err := GetDataFromMemory(instanceMemory, key)
				if err != nil {
					return "", err
				}
				return string(data), nil
			}
			return "", err
		}
		return str, nil
	}

	if define.IN_MEMORY_CACHE {
		data, err := GetDataFromMemory(instanceMemory, key)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", NO_CACHE_ENABLED
}

func Set(key string, value string) error {
	if define.REDIS {
		err := UpdateDataToRedis(key, value)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				UpdateDataToMemory(instanceMemory, key, value)
				return nil
			}
			return err
		}
	}

	if define.IN_MEMORY_CACHE {
		UpdateDataToMemory(instanceMemory, key, value)
		return nil
	}
	return NO_CACHE_ENABLED
}

func Del(key string) error {
	if define.REDIS {
		err := DelDataByKeyFromRedis(key)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				DelDataByKeyFromMemory(instanceMemory, key)
				return nil
			}
			return err
		}
	}

	if define.IN_MEMORY_CACHE {
		DelDataByKeyFromMemory(instanceMemory, key)
		return nil
	}
	return NO_CACHE_ENABLED
}

func Expire(key string, expire time.Duration) error {
	if define.REDIS {
		err := SetDataExpireByKeyFromRedis(key, expire*time.Second)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				err := SetDataExpireByKeyFromMemory(instanceMemory, key, expire*time.Second)
				if err != nil {
					return err
				}
				return nil
			}
			return err
		}
	}

	if define.IN_MEMORY_CACHE {
		err := SetDataExpireByKeyFromMemory(instanceMemory, key, expire*time.Second)
		if err != nil {
			return err
		}
		return nil
	}
	return NO_CACHE_ENABLED
}

func IsEnable() bool {
	return define.IN_MEMORY_CACHE || define.REDIS
}
