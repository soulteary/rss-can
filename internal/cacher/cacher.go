package cacher

import (
	"errors"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
)

var NO_CACHE_ENABLED = (func() error {
	return errors.New("no cache enabled")
})()

func Get(key string) (string, error) {
	if define.REDIS_ENABLED {
		str, err := GetDataFromRedis(key)
		if err != nil {
			if define.MEMORY_CACHE_ENABLED {
				data, err := GetDataFromMemory(key)
				if err != nil {
					return "", err
				}
				return string(data), nil
			}
			return "", err
		}
		return str, nil
	}

	if define.MEMORY_CACHE_ENABLED {
		data, err := GetDataFromMemory(key)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", NO_CACHE_ENABLED
}

func Set(key string, value string) error {
	if define.REDIS_ENABLED {
		err := UpdateDataToRedis(key, value)
		if err != nil {
			if define.MEMORY_CACHE_ENABLED {
				UpdateDataToMemory(key, value)
				return nil
			}
			return err
		}
	}

	if define.MEMORY_CACHE_ENABLED {
		UpdateDataToMemory(key, value)
		return nil
	}
	return NO_CACHE_ENABLED
}

func Del(key string) error {
	if define.REDIS_ENABLED {
		err := DelDataByKeyFromRedis(key)
		if err != nil {
			if define.MEMORY_CACHE_ENABLED {
				DelDataByKeyFromMemory(key)
				return nil
			}
			return err
		}
	}

	if define.MEMORY_CACHE_ENABLED {
		DelDataByKeyFromMemory(key)
		return nil
	}
	return NO_CACHE_ENABLED
}

func Expire(key string, expire time.Duration) error {
	if define.REDIS_ENABLED {
		err := SetDataExpireByKeyFromRedis(key, expire)
		if err != nil {
			if define.MEMORY_CACHE_ENABLED {
				err := SetDataExpireByKeyFromMemory(key, expire)
				if err != nil {
					return err
				}
				return nil
			}
			return err
		}
	}

	if define.MEMORY_CACHE_ENABLED {
		err := SetDataExpireByKeyFromMemory(key, expire)
		if err != nil {
			return err
		}
		return nil
	}
	return NO_CACHE_ENABLED
}

func IsEnable() bool {
	return define.MEMORY_CACHE_ENABLED || define.REDIS_ENABLED
}
