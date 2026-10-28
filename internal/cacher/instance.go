package cacher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config is captured at construction; an application cache never consults
// mutable command-line globals after startup.
type Config struct {
	RedisEnabled, MemoryEnabled bool
	RedisAddr, RedisPassword    string
	RedisDB                     int
	MemoryTTL                   time.Duration
}

type Cache struct {
	config   Config
	redis    *redis.Client
	service  *Service
	once     sync.Once
	closeErr error
}

func New(config Config) *Cache {
	if config.MemoryTTL <= 0 {
		config.MemoryTTL = 10 * time.Minute
	}
	if config.RedisAddr == "" {
		config.RedisAddr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: config.RedisAddr, Password: config.RedisPassword, DB: config.RedisDB, PoolSize: 100, PipelinePoolSize: -1, MaxRetries: 3, ContextTimeoutEnabled: true})
	memory := newMemoryBackend(newMemory(config.MemoryTTL))
	return &Cache{config: config, redis: client, service: NewService(newRedisBackend(client), memory, func() bool { return config.RedisEnabled }, func() bool { return config.MemoryEnabled })}
}
func (c *Cache) Close() error {
	c.once.Do(func() { c.closeErr = c.service.Close() })
	return c.closeErr
}
func (c *Cache) Check(ctx context.Context) Status {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	return check(ctx, c.config.RedisEnabled, c.config.MemoryEnabled, func(ctx context.Context) error { return c.redis.Ping(ctx).Err() })
}

type cacheContextKey struct{}

// WithCache bridges the existing context-aware parser APIs to the application
// owned cache without changing the public legacy helpers all at once.
func WithCache(ctx context.Context, cache *Cache) context.Context {
	return context.WithValue(ctx, cacheContextKey{}, cache)
}
func scopedCache(ctx context.Context) *Cache {
	cache, _ := ctx.Value(cacheContextKey{}).(*Cache)
	return cache
}
func Scope(ctx context.Context) string {
	if cache := scopedCache(ctx); cache != nil {
		return fmt.Sprintf("%p", cache)
	}
	return "legacy"
}
func IsEnabledContext(ctx context.Context) bool {
	if cache := scopedCache(ctx); cache != nil {
		return cache.service.Enabled()
	}
	return IsEnable()
}
