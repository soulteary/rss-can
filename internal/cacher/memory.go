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

func init() {
}

func InitializeMemory(enabled bool, store string) *cache2go.CacheTable {
	if enabled {
		return cache2go.Cache(store)
	}
	return nil
}

func IsMemoryEmpty(instance *cache2go.CacheTable) bool {
	return instance.Count() == 0
}

func UpdateDataToMemory(instance *cache2go.CacheTable, key, value string) {
	now := time.Now()
	val := InMemoryPageCache{now, []byte(value)}
	instance.Add(key, fn.I2T(define.IN_MEMORY_EXPIRATION)*time.Second, &val)
}

func GetDataFromMemory(instance *cache2go.CacheTable, key string) ([]byte, error) {
	if IsMemoryEmpty(instance) {
		return []byte(""), nil
	}
	res, err := instance.Value(key)
	if err != nil {
		logger.Instance.Errorf("Error retrieving value from cache: %v", err)
		return []byte(""), err
	}
	return res.Data().(*InMemoryPageCache).page, nil
}

func DelDataByKeyFromMemory(instance *cache2go.CacheTable, key string) {
	if IsMemoryEmpty(instance) {
		return
	}
	instance.Delete(key)
}

func SetDataExpireByKeyFromMemory(instance *cache2go.CacheTable, key string, expire time.Duration) error {
	data, err := GetDataFromMemory(instance, key)
	if err != nil {
		logger.Instance.Errorf("Error retrieving value from cache: %v", err)
		return err
	}
	now := time.Now()
	val := InMemoryPageCache{now, data}
	instance.Add(key, expire, &val)
	return nil
}

func FlushDataFromMemory(instance *cache2go.CacheTable) {
	instance.Flush()
}
