package cacher

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestUnavailableRedisFallsBackWithoutReplacingClient(t *testing.T) {
	logger.Initialize()
	Initialize()
	original := instanceRedis
	originalService := instanceService
	redisEnabled, memoryEnabled := define.REDIS, define.IN_MEMORY_CACHE
	failed := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	failed.Close() // Deterministic outage; no network or connection retries.
	instanceRedis = failed
	instanceService = NewService(newRedisBackend(failed), originalService.fallback,
		func() bool { return define.REDIS }, func() bool { return define.IN_MEMORY_CACHE })
	define.REDIS = true
	define.IN_MEMORY_CACHE = true
	defer func() {
		instanceRedis = original
		instanceService = originalService
		define.REDIS = redisEnabled
		define.IN_MEMORY_CACHE = memoryEnabled
	}()
	key := t.Name()
	if err := Set(key, "fallback"); err != nil {
		t.Fatal(err)
	}
	if value, err := Get(key); err != nil || value != "fallback" {
		t.Fatalf("fallback: %q %v", value, err)
	}
	if err := Expire(key, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := Del(key); err != nil {
		t.Fatal(err)
	}
	if instanceRedis != failed {
		t.Fatal("failed client replaced; configured pool lost")
	}
	define.IN_MEMORY_CACHE = false
	if _, err := Get(key); err == nil {
		t.Fatal("Redis failure hidden without a fallback backend")
	}
	if err := Set(key, "value"); err == nil {
		t.Fatal("Redis write failure hidden")
	}
}
