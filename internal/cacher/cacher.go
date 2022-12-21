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
	if define.REDIS {
		str, err := GetDataFromRedis(key)
		if err != nil {
			if define.IN_MEMORY_CACHE {
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

	if define.IN_MEMORY_CACHE {
		data, err := GetDataFromMemory(key)
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
				UpdateDataToMemory(key, value)
				return nil
			}
			return err
		}
	}

	if define.IN_MEMORY_CACHE {
		UpdateDataToMemory(key, value)
		return nil
	}
	return NO_CACHE_ENABLED
}

func Del(key string) error {
	if define.REDIS {
		err := DelDataByKeyFromRedis(key)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				DelDataByKeyFromMemory(key)
				return nil
			}
			return err
		}
	}

	if define.IN_MEMORY_CACHE {
		DelDataByKeyFromMemory(key)
		return nil
	}
	return NO_CACHE_ENABLED
}

func Expire(key string, expire time.Duration) error {
	if define.REDIS {
		err := SetDataExpireByKeyFromRedis(key, expire*time.Second)
		if err != nil {
			if define.IN_MEMORY_CACHE {
				err := SetDataExpireByKeyFromMemory(key, expire*time.Second)
				if err != nil {
					return err
				}
				return nil
			}
			return err
		}
	}

	if define.IN_MEMORY_CACHE {
		err := SetDataExpireByKeyFromMemory(key, expire*time.Second)
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
