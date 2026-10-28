package cacher

import (
	"context"
	"testing"
	"time"
)

func TestApplicationCachesKeepConfigurationAndValuesIndependent(t *testing.T) {
	a := New(Config{MemoryEnabled: true, MemoryTTL: time.Hour, RedisAddr: "127.0.0.1:16381", RedisPassword: "a", RedisDB: 1})
	b := New(Config{MemoryEnabled: true, MemoryTTL: time.Minute, RedisAddr: "127.0.0.1:16382", RedisPassword: "b", RedisDB: 2})
	defer a.Close()
	defer b.Close()
	ca, cb := WithCache(context.Background(), a), WithCache(context.Background(), b)
	if Scope(ca) == Scope(cb) || Scope(context.Background()) != "legacy" {
		t.Fatal("application cache scope collided")
	}
	for _, entry := range []struct {
		ctx   context.Context
		value string
	}{{ca, "a"}, {cb, "b"}} {
		if err := SetWithTTL(entry.ctx, "shared-key", entry.value, 0); err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range []struct {
		ctx   context.Context
		value string
	}{{ca, "a"}, {cb, "b"}} {
		value, found, err := GetWithStatus(entry.ctx, "shared-key")
		if err != nil || !found || value != entry.value {
			t.Fatalf("crossed cache instances: %q %v %v", value, found, err)
		}
	}
	if a.redis.Options().Addr == b.redis.Options().Addr || a.redis.Options().DB != 1 || b.redis.Options().Password != "b" {
		t.Fatal("client configuration crossed instance boundaries")
	}
	if status := Check(ca); !status.Ready || status.Backend != "memory" {
		t.Fatalf("scoped readiness: %+v", status)
	}
	if !IsEnabledContext(ca) {
		t.Fatal("scoped cache disabled")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := SetWithTTL(cb, "still-open", "b", time.Second); err != nil {
		t.Fatalf("closing a closed b: %v", err)
	}
	disabled := New(Config{})
	defer disabled.Close()
	if IsEnabledContext(WithCache(context.Background(), disabled)) {
		t.Fatal("constructed disabled cache inherited globals")
	}
}
