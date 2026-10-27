package cacher

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/soulteary/RSS-Can/internal/define"
	rediscache "github.com/soulteary/redis-kit/cache"
	redisclient "github.com/soulteary/redis-kit/client"
)

func InitializeRedis(enabled bool) *redis.Client {
	if !enabled {
		return nil
	}
	// Keep client construction lazy: the cacher can fall back to memory when
	// Redis is unavailable, so initialization must not require a successful PING.
	return redis.NewClient(&redis.Options{
		Addr:     define.REDIS_SERVER,
		Password: define.REDIS_PASS,
		DB:       define.REDIS_DB,
		PoolSize: 100,
		// Keep pipelines inside the configured connection budget, matching
		// clients constructed by redis-kit.
		PipelinePoolSize: -1,
		MaxRetries:       3,
	})
}

func Disconnect(instance *redis.Client) error {
	return redisclient.Close(instance)
}

func UpdateDataToRedis(instance *redis.Client, key string, value string) error {
	if instance == nil {
		return errors.New("redis cache is not initialized")
	}
	return rediscache.NewCache(instance, "").Set(context.Background(), key, value, 0)
}

func GetDataFromRedis(instance *redis.Client, key string) (string, error) {
	if instance == nil {
		return "", errors.New("redis cache is not initialized")
	}

	ctx := context.Background()
	var result string
	err := rediscache.NewCache(instance, "").Get(ctx, key, &result)
	if errors.Is(err, rediscache.ErrKeyNotFound) {
		return "", nil
	}
	if err == nil {
		return result, nil
	}

	// redis-kit JSON-encodes cached strings. Values created by older RSS-Can
	// releases are raw strings, so retain a read-only migration path for them.
	legacy, legacyErr := instance.Get(ctx, key).Result()
	if errors.Is(legacyErr, redis.Nil) {
		return "", nil
	}
	if legacyErr != nil {
		return "", err
	}
	return legacy, nil
}

func DelDataByKeyFromRedis(instance *redis.Client, key string) error {
	if instance == nil {
		return errors.New("redis cache is not initialized")
	}
	return rediscache.NewCache(instance, "").Del(context.Background(), key)
}

func SetDataExpireByKeyFromRedis(instance *redis.Client, key string, expire time.Duration) error {
	if instance == nil {
		return errors.New("redis cache is not initialized")
	}
	return rediscache.NewCache(instance, "").Expire(context.Background(), key, expire)
}
