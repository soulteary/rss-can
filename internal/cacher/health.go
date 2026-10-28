package cacher

import (
	"context"
	"github.com/soulteary/RSS-Can/internal/define"
	"time"
)

type Status struct {
	Ready    bool   `json:"ready"`
	Backend  string `json:"backend"`
	Degraded bool   `json:"degraded"`
}

func Check(ctx context.Context) Status {
	if cache := scopedCache(ctx); cache != nil {
		return cache.Check(ctx)
	}
	Initialize()
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	return check(ctx, define.REDIS, define.IN_MEMORY_CACHE, func(ctx context.Context) error { return instanceRedis.Ping(ctx).Err() })
}

func check(ctx context.Context, redisEnabled, memoryEnabled bool, ping func(context.Context) error) Status {
	if redisEnabled {
		if ping(ctx) == nil {
			return Status{Ready: true, Backend: "redis"}
		}
		if memoryEnabled {
			return Status{Ready: true, Backend: "memory", Degraded: true}
		}
		return Status{Backend: "redis", Degraded: true}
	}
	if memoryEnabled {
		return Status{Ready: true, Backend: "memory"}
	}
	return Status{Ready: true, Backend: "disabled"}
}
