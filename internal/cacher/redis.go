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

type redisBackend struct {
	client *redis.Client
	cache  *rediscache.RedisCache
}

func newRedisBackend(client *redis.Client) *redisBackend {
	return &redisBackend{client: client, cache: rediscache.NewCache(client, redisKitKeyPrefix)}
}

func (b *redisBackend) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if b.client == nil {
		return errors.New("redis cache is not initialized")
	}
	return b.cache.Set(ctx, key, value, ttl)
}

func (b *redisBackend) Get(ctx context.Context, key string) (string, bool, error) {
	if b.client == nil {
		return "", false, errors.New("redis cache is not initialized")
	}
	var result string
	err := b.cache.Get(ctx, key, &result)
	if errors.Is(err, rediscache.ErrKeyNotFound) {
		legacy, legacyErr := b.client.Get(ctx, key).Result()
		if errors.Is(legacyErr, redis.Nil) {
			return "", false, nil
		}
		if legacyErr != nil {
			return "", false, legacyErr
		}
		return legacy, true, nil
	}
	if err != nil {
		return "", false, err
	}
	return result, true, nil
}

func (b *redisBackend) Delete(ctx context.Context, key string) error {
	if b.client == nil {
		return errors.New("redis cache is not initialized")
	}
	if err := b.cache.Del(ctx, key); err != nil {
		return err
	}
	return b.client.Del(ctx, key).Err()
}

func (b *redisBackend) Close() error {
	return redisclient.Close(b.client)
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
	return newRedisBackend(instance).Set(context.Background(), key, value, 0)
}

func GetDataFromRedis(instance *redis.Client, key string) (string, error) {
	value, _, err := newRedisBackend(instance).Get(context.Background(), key)
	return value, err
}

func DelDataByKeyFromRedis(instance *redis.Client, key string) error {
	return newRedisBackend(instance).Delete(context.Background(), key)
}

func SetDataExpireByKeyFromRedis(instance *redis.Client, key string, expire time.Duration) error {
	ctx := context.Background()
	backend := newRedisBackend(instance)
	if instance == nil {
		return errors.New("redis cache is not initialized")
	}
	if err := backend.cache.Expire(ctx, key, expire); err != nil {
		return err
	}
	return instance.Expire(ctx, key, expire).Err()
}
