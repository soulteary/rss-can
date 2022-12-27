package cacher

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/soulteary/RSS-Can/internal/define"
)

func InitializeRedis(enabled bool) *redis.Client {
	if !enabled {
		return nil
	}
	instance := redis.NewClient(&redis.Options{
		Addr:       define.REDIS_SERVER,
		Password:   define.REDIS_PASS,
		DB:         define.REDIS_DB,
		PoolSize:   100,
		MaxRetries: 3,
	})
	return instance
}

func Connect(instance *redis.Client) *redis.Client {
	var ctx = context.Background()

	if instance == nil {
		return nil
	}

	err := instance.Ping(ctx).Err()
	if err == nil {
		return instance
	}
	instance.Close()

	return InitializeRedis(define.REDIS)
}

func Disconnect(instance *redis.Client) (err error) {
	return instance.Close()
}

func UpdateDataToRedis(instance *redis.Client, key string, value string) (err error) {
	var ctx = context.Background()

	rdb := Connect(instance)
	err = rdb.Set(ctx, key, value, 0).Err()
	if err != nil {
		return err
	}
	return nil
}

func GetDataFromRedis(instance *redis.Client, key string) (result string, err error) {
	var ctx = context.Background()

	rdb := Connect(instance)
	data, err := rdb.Get(ctx, key).Result()
	// REDIS_KEY_NOT_EXIST
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	} else {
		return data, nil
	}
}

func DelDataByKeyFromRedis(instance *redis.Client, key string) (err error) {
	var ctx = context.Background()

	rdb := Connect(instance)
	_, err = rdb.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	return nil
}

func SetDataExpireByKeyFromRedis(instance *redis.Client, key string, expire time.Duration) (err error) {
	var ctx = context.Background()

	rdb := Connect(instance)
	_, err = rdb.Expire(ctx, key, expire).Result()
	if err != nil {
		return err
	}
	return nil
}
