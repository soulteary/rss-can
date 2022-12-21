package cacher

import (
	"time"

	"github.com/muesli/cache2go"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
)

type InMemoryPageCache struct {
	CreateAt time.Time
	page     []byte
}

var instanceMemory *cache2go.CacheTable

func init() {
	if define.MEMORY_CACHE_ENABLED {
		instanceMemory = cache2go.Cache(define.IN_MEMORY_CACHE_STORE_NAME)
	}
}

func UpdateDataToMemory(key, value string) {
	now := time.Now()
	val := InMemoryPageCache{now, []byte(value)}
	instanceMemory.Add(key, fn.I2T(define.IN_MEMORY_CACHE_EXPIRATION)*time.Second, &val)
}

func GetDataFromMemory(key string) ([]byte, error) {
	res, err := instanceMemory.Value(key)
	if err != nil {
		logger.Instance.Errorf("Error retrieving value from cache: %v", err)
		return []byte(""), err
	}
	return res.Data().(*InMemoryPageCache).page, nil
}

func DelDataByKeyFromMemory(key string) {
	instanceMemory.Delete(key)
}

func SetDataExpireByKeyFromMemory(key string, expire time.Duration) error {
	data, err := GetDataFromMemory(key)
	if err != nil {
		logger.Instance.Errorf("Error retrieving value from cache: %v", err)
		return err
	}
	now := time.Now()
	val := InMemoryPageCache{now, data}
	instanceMemory.Add(key, expire, &val)
	return nil
}

func FlushDataFromMemory() {
	instanceMemory.Flush()
}
