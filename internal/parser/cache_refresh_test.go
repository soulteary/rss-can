package parser

import (
	"context"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/network"
)

func TestBrowserRefreshReplacesCache(t *testing.T) {
	cache := cacher.New(cacher.Config{MemoryEnabled: true, MemoryTTL: time.Hour})
	defer cache.Close()
	ctx := cacher.WithCache(t.Context(), cache)
	calls := 0
	fetch := func(context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "old", nil
		}
		return "new", nil
	}
	for _, tc := range []struct {
		ctx  context.Context
		want string
	}{{ctx, "old"}, {network.WithCacheRefresh(ctx), "new"}, {ctx, "new"}} {
		got, err := browserData(tc.ctx, "refresh-test", true, time.Second, 3600, fetch)
		if err != nil || got != tc.want {
			t.Fatalf("got=%q err=%v want=%q", got, err, tc.want)
		}
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}
