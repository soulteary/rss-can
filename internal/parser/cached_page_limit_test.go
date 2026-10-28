package parser

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func browserCacheContext(t *testing.T) context.Context {
	t.Helper()
	logger.Initialize()
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	t.Cleanup(func() { cache.Close() })
	return cacher.WithCache(t.Context(), cache)
}
func TestBrowserCachedOversizeRefetchesBeforeCSRAndMixParsing(t *testing.T) {
	for _, mode := range []string{"csr", "mix"} {
		t.Run(mode, func(t *testing.T) {
			ctx := browserCacheContext(t)
			key := "browser:test:" + mode
			if err := cacher.SetWithTTL(ctx, key, strings.Repeat("x", limits.MaxPageBytes+1), time.Minute); err != nil {
				t.Fatal(err)
			}
			calls := 0
			fresh := `[{"title":"fresh","link":"https://example.org/post"}]`
			if mode == "mix" {
				fresh = `<article><h2>fresh</h2><a href="/post">read</a></article>`
			}
			fetch := func(context.Context) (string, error) { calls++; return fresh, nil }
			for range 2 {
				data, err := browserData(ctx, key, true, time.Second, 60, fetch)
				if err != nil || data != fresh {
					t.Fatalf("cache result %d %v", len(data), err)
				}
				var parsed define.BodyParsed
				if mode == "csr" {
					parsed = parseCSRItems(data)
				} else {
					parsed = ParseDataAndConfigBySSRContext(ctx, define.JavaScriptConfig{URL: "https://example.org", ListContainer: "article", Title: "h2", Link: "a"}, define.RemoteBodySanitized{}, data)
				}
				if parsed.Code != define.ERROR_CODE_NULL || len(parsed.Body) != 1 || parsed.Body[0].Title != "fresh" {
					t.Fatalf("parsing %s: %+v", mode, parsed)
				}
			}
			if calls != 1 {
				t.Fatalf("browser fetches=%d", calls)
			}
		})
	}
}
func TestBrowserCachedOversizeCannotReturnPartialOrOversizeReplacement(t *testing.T) {
	ctx := browserCacheContext(t)
	key := "browser:test:invalid"
	oversized := strings.Repeat("x", limits.MaxPageBytes+1)
	if err := cacher.SetWithTTL(ctx, key, oversized, time.Minute); err != nil {
		t.Fatal(err)
	}
	data, err := browserData(ctx, key, true, time.Second, 60, func(context.Context) (string, error) { return oversized, nil })
	if err == nil || data != "" {
		t.Fatalf("oversized replacement returned: %d %v", len(data), err)
	}
	if _, found, _ := cacher.GetWithStatus(ctx, key); found {
		t.Fatal("oversized value remained cached")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	data, err = browserData(canceled, key, true, time.Second, 60, func(context.Context) (string, error) {
		t.Fatal("canceled request started a browser fetch")
		return "", nil
	})
	if !errors.Is(err, context.Canceled) || data != "" {
		t.Fatalf("canceled request %q %v", data, err)
	}
}

func TestBrowserRuleCaptureHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := captureBrowserRuleContext(ctx, define.JavaScriptConfig{File: "missing.js"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled capture: %v", err)
	}
}
