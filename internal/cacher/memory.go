package cacher

import (
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/cache-kit/v2/ttlcache"
)

func InitializeMemory(enabled bool, _ string) *ttlcache.Memory[string, []byte] {
	if enabled {
		return ttlcache.New(ttlcache.Config[string, []byte]{
			DefaultTTL: fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION),
		})
	}
	return nil
}

func IsMemoryEmpty(instance *ttlcache.Memory[string, []byte]) bool {
	return instance.Len() == 0
}

func UpdateDataToMemory(instance *ttlcache.Memory[string, []byte], key, value string) {
	_ = instance.Set(key, []byte(value), 0)
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
