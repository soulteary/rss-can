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

const redisKitKeyPrefix = "rss-can:redis-kit:v1:"

func redisCache(instance *redis.Client) *rediscache.RedisCache {
	return rediscache.NewCache(instance, redisKitKeyPrefix)
}

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
	return redisCache(instance).Set(context.Background(), key, value, 0)
}

func GetDataFromRedis(instance *redis.Client, key string) (string, error) {
	if instance == nil {
		return "", errors.New("redis cache is not initialized")
	}

	ctx := context.Background()
	var result string
	err := redisCache(instance).Get(ctx, key, &result)
	if errors.Is(err, rediscache.ErrKeyNotFound) {
		// Releases before redis-kit stored arbitrary response bodies as raw Redis
		// strings. A separate namespace is required because a legacy body may
		// itself be valid JSON and cannot be distinguished by decoding it.
		legacy, legacyErr := instance.Get(ctx, key).Result()
		if errors.Is(legacyErr, redis.Nil) {
			return "", nil
		}
		if legacyErr != nil {
			return "", legacyErr
		}
		return legacy, nil
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

func DelDataByKeyFromRedis(instance *redis.Client, key string) error {
	if instance == nil {
		return errors.New("redis cache is not initialized")
	}
	ctx := context.Background()
	if err := redisCache(instance).Del(ctx, key); err != nil {
		return err
	}
	return instance.Del(ctx, key).Err()
}

func SetDataExpireByKeyFromRedis(instance *redis.Client, key string, expire time.Duration) error {
	if instance == nil {
		return errors.New("redis cache is not initialized")
	}
	ctx := context.Background()
	if err := redisCache(instance).Expire(ctx, key, expire); err != nil {
		return err
	}
	return instance.Expire(ctx, key, expire).Err()
}
