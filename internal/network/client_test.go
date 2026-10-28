package network

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestNetworkClientsDoNotMergeWorkAcrossInstances(t *testing.T) {
	logger.Initialize()
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			close(entered)
			<-release
		}
		fmt.Fprintf(w, "response-%d", n)
	}))
	defer upstream.Close()
	// Deliberately share a backing cache and configuration to ensure isolation is
	// provided by each client, rather than merely differing keys or credentials.
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	defer cache.Close()
	a, b := NewClient(Config{}), NewClient(Config{})
	ca := WithClient(cacher.WithCache(context.Background(), cache), a)
	cb := WithClient(cacher.WithCache(context.Background(), cache), b)
	first := make(chan define.RemoteBodySanitized, 1)
	go func() { first <- GetRemoteDocumentContext(ca, upstream.URL, "", 60, false) }()
	<-entered
	second := make(chan define.RemoteBodySanitized, 1)
	go func() { second <- GetRemoteDocumentContext(cb, upstream.URL, "", 60, false) }()
	select {
	case result := <-second:
		close(release)
		if result.Code != 0 || result.Body != "response-2" {
			t.Fatalf("second client result: %+v", result)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("independent client joined the first client's blocked fetch")
	}
	if result := <-first; result.Code != 0 || result.Body != "response-1" {
		t.Fatalf("first client result: %+v", result)
	}
}

func TestScopedNetworkSettingsAndCacheCredentials(t *testing.T) {
	logger.Initialize()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, r.UserAgent()) }))
	defer upstream.Close()
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	defer cache.Close()
	ctx := cacher.WithCache(context.Background(), cache)
	a := NewClient(Config{UserAgent: "app-a", RequestTimeout: time.Second, ServerTimeout: 2 * time.Second, DefaultTTL: time.Minute})
	b := NewClient(Config{UserAgent: "app-b", RequestTimeout: 2 * time.Second})
	for _, entry := range []struct {
		client *Client
		want   string
	}{{a, "app-a"}, {b, "app-b"}, {a, "app-a"}} {
		result := GetRemoteDocumentContext(WithClient(ctx, entry.client), upstream.URL, "", 0, false)
		if result.Code != 0 || result.Body != entry.want {
			t.Fatalf("scoped user-agent/cache identity: %+v", result)
		}
	}
	if got := configForContext(WithClient(ctx, a)); got.RequestTimeout != time.Second || got.ServerTimeout != 2*time.Second || got.DefaultTTL != time.Minute {
		t.Fatalf("captured settings changed: %+v", got)
	}
}

func TestScopedProxyIgnoresGlobalFallback(t *testing.T) {
	previous := define.PROXY_SERVER
	define.PROXY_SERVER = "http://legacy.invalid:8080"
	defer func() { define.PROXY_SERVER = previous }()
	direct := WithClient(context.Background(), NewClient(Config{}))
	configured := WithClient(context.Background(), NewClient(Config{Proxy: "http://app.invalid:8080"}))
	if got := OptionsForConfigContext(direct, define.JavaScriptConfig{}).Proxy; got != "" {
		t.Fatalf("empty app proxy inherited global: %s", got)
	}
	if got := OptionsForConfigContext(configured, define.JavaScriptConfig{}).Proxy; got != "http://app.invalid:8080" {
		t.Fatalf("app proxy missing: %s", got)
	}
	if got := OptionsForConfigContext(configured, define.JavaScriptConfig{Proxy: "http://rule.invalid:8080"}).Proxy; got != "http://rule.invalid:8080" {
		t.Fatalf("rule proxy overridden: %s", got)
	}
	if got := OptionsForConfigContext(context.Background(), define.JavaScriptConfig{}).Proxy; got != define.PROXY_SERVER {
		t.Fatalf("legacy fallback changed: %s", got)
	}
	if Scope(direct) == Scope(configured) || Scope(context.Background()) != "legacy" {
		t.Fatal("network client scopes collided")
	}
}
