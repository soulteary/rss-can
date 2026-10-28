package parser

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func memoryCacheForTest(t *testing.T) {
	t.Helper()
	logger.Initialize()
	redis, memory := define.REDIS, define.IN_MEMORY_CACHE
	define.REDIS = false
	define.IN_MEMORY_CACHE = true
	t.Cleanup(func() { define.REDIS = redis; define.IN_MEMORY_CACHE = memory })
}

func TestCSRCacheDecodesJSONAndSeparatesModes(t *testing.T) {
	memoryCacheForTest(t)
	config := define.JavaScriptConfig{URL: "https://example.org/" + t.Name(), ListContainer: "article", Title: "h2", Link: "a"}
	jsonData := `[{"title":"Cached title","link":"https://example.org/item","date":""}]`
	if err := cacher.Set(browserCacheKey(config, false), jsonData); err != nil {
		t.Fatal(err)
	}
	cached := ParsePageByBrowser(config, "unreachable-browser", "", false)
	if cached.Code != 0 || len(cached.Body) != 1 || cached.Body[0].Title != "Cached title" {
		t.Fatalf("JSON cache parsed as HTML: %#v", cached)
	}
	if err := cacher.Set(browserCacheKey(config, true), `<article><h2>Mixed title</h2><a href="/item">Read</a></article>`); err != nil {
		t.Fatal(err)
	}
	mixed := ParsePageByBrowser(config, "unreachable-browser", "", true)
	if mixed.Code != 0 || len(mixed.Body) != 1 || mixed.Body[0].Title != "Mixed title" {
		t.Fatalf("HTML cache not parsed correctly: %#v", mixed)
	}
	// An old plain-URL SSR cache entry must not replace the extracted CSR JSON.
	if err := cacher.Set(config.URL, "wrong representation"); err != nil {
		t.Fatal(err)
	}
	again := ParsePageByBrowser(config, "unreachable-browser", "", false)
	if !reflect.DeepEqual(again.Body, cached.Body) {
		t.Fatal("SSR and CSR caches collided")
	}
	if got := parseCSRItems("not-json"); got.Code == 0 {
		t.Fatal("invalid CSR result treated as success")
	}
	changed := config
	changed.Cookies = "session=different"
	if browserCacheKey(changed, false) == browserCacheKey(config, false) {
		t.Fatal("cookie-specific caches collided")
	}
}

func TestBrowserConnectionAndQueueTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	if _, err := resolveBrowserURL(ctx, server.URL); err == nil {
		t.Fatal("discovery did not time out")
	}
	for i := 0; i < cap(browserSlots); i++ {
		browserSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(browserSlots); i++ {
			<-browserSlots
		}
	}()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel2()
	if err := withBrowserPage(ctx2, "unreachable", "", "", "", false, func(context.Context) error { t.Fatal("full queue admitted browser work"); return nil }); err == nil {
		t.Fatal("full queue did not time out")
	}
}

func TestBrowserWebSocketHandshakeTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var b [4096]byte
		for {
			if _, err := conn.Read(b[:]); err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := withBrowserPage(ctx, "ws://"+listener.Addr().String()+"/devtools/browser/stalled", "", "", "", false, func(context.Context) error { t.Error("stalled handshake succeeded"); return nil }); err == nil {
		t.Fatal("handshake did not time out")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handshake socket leaked")
	}
}

func TestBrowserLifecycleAndLiveCache(t *testing.T) {
	address := os.Getenv("RSS_TEST_BROWSER_URL")
	if address == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("RSS_TEST_BROWSER_URL is required for browser regression tests")
		}
		t.Skip("set RSS_TEST_BROWSER_URL to a Chrome remote debugging endpoint to enable browser integration tests")
	}
	memoryCacheForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	websocketURL, err := resolveBrowserURL(ctx, address)
	if err != nil {
		t.Fatal(err)
	}
	allocCtx, allocCancel := remote.NewAllocator(ctx, websocketURL, remote.NoModifyURL)
	defer allocCancel()
	controllerCtx, controllerCancel := chromedp.NewContext(allocCtx)
	defer controllerCancel()
	if err := chromedp.Do(controllerCtx); err != nil {
		t.Fatal(err)
	}
	assertClean := func(t *testing.T) {
		t.Helper()
		contexts, err := cdp.Call(controllerCtx, chromedp.FromContext(controllerCtx).Browser, target.GetBrowserContexts, cdp.Empty{})
		if err != nil {
			t.Fatal(err)
		}
		if len(contexts.BrowserContextIDs) != 0 {
			t.Fatalf("browser contexts leaked: %v", contexts.BrowserContextIDs)
		}
		if len(browserSlots) != 0 {
			t.Fatal("browser concurrency slot leaked")
		}
	}
	var requests atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `<html><body><article><h2>Live title</h2><a href="https://example.org/item">Read</a></article></body></html>`)
	}))
	defer upstream.Close()
	ruleFile := filepath.Join(t.TempDir(), "rule.js")
	rule := []byte(`potted.SetScope("article").SetConfig({Title:"h2",Link:"a"});`)
	if err := os.WriteFile(ruleFile, rule, 0600); err != nil {
		t.Fatal(err)
	}
	config := define.JavaScriptConfig{URL: upstream.URL, File: ruleFile, ListContainer: "article", Title: "h2", Link: "a", Timeout: 5}
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache mixed=%t", mixed), func(t *testing.T) {
			first := ParsePageByBrowser(config, address, "", mixed)
			if first.Code != 0 || len(first.Body) != 1 || first.Body[0].Title != "Live title" {
				t.Fatalf("live parse: %#v", first)
			}
			count := requests.Load()
			cached := ParsePageByBrowser(config, "ws://127.0.0.1:1", "", mixed)
			if cached.Code != 0 || !reflect.DeepEqual(cached.Body, first.Body) {
				t.Fatalf("cached parse: %#v; live: %#v", cached, first)
			}
			if requests.Load() != count {
				t.Fatal("cache hit fetched upstream again")
			}
			assertClean(t)
		})
	}
	t.Run("SDK jQuery is isolated from target page", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := withBrowserPage(ctx, address, "", upstream.URL, "", false, func(pageCtx context.Context) error {
			if err := chromedp.Do(pageCtx, chromedp.Navigate(upstream.URL)); err != nil {
				return err
			}
			if _, err := chromedp.Run(pageCtx, chromedp.Evaluate[chromedp.Void](`window.originalDollar = window.$ = {pageOwned: true}; window.originalJQuery = window.jQuery = {pageOwned: true};`)); err != nil {
				return err
			}
			sdkRule := append(append([]byte{}, rule...), []byte(`
    if (typeof jQuery !== "function" || jQuery.fn.jquery !== "4.0.0" || $ !== jQuery || POTTED.fn.$ !== $) {
     throw new Error("private jQuery aliases are missing or inconsistent");
    }
   `)...)
			code := jssdk.GenerateCSRInjectParser(sdkRule)
			for i := 0; i < 2; i++ {
				value, err := chromedp.Run(pageCtx, chromedp.Evaluate[string]("("+code+")()"))
				if err != nil {
					return err
				}
				result := parseCSRItems(value)
				if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Title != "Live title" {
					return fmt.Errorf("SDK result: %#v", result)
				}
				preserved, err := chromedp.Run(pageCtx, chromedp.Evaluate[bool](`window.$ === window.originalDollar && window.jQuery === window.originalJQuery`))
				if err != nil {
					return err
				}
				if !preserved {
					return fmt.Errorf("SDK replaced the page's jQuery globals")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		assertClean(t)
	})
	t.Run("cleanup after navigation timeout", func(t *testing.T) {
		broken := config
		broken.DisableCache = true
		broken.Timeout = 1
		broken.ListContainer = ".missing"
		result := ParsePageByBrowser(broken, address, "", false)
		if result.Code == 0 {
			t.Fatal("missing selector treated as success")
		}
		assertClean(t)
	})
	t.Run("cleanup after evaluation timeout", func(t *testing.T) {
		if err := os.WriteFile(ruleFile, []byte(`potted.GetData = () => { while(true) {} };`), 0600); err != nil {
			t.Fatal(err)
		}
		broken := config
		broken.DisableCache = true
		broken.Timeout = 1
		result := ParsePageByBrowser(broken, address, "", false)
		if result.Code == 0 {
			t.Fatal("infinite browser script treated as success")
		}
		assertClean(t)
	})
	t.Run("cleanup after action error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := withBrowserPage(ctx, address, "", upstream.URL, "", false, func(context.Context) error { return fmt.Errorf("test failure") }); err == nil {
			t.Fatal("action error swallowed")
		}
		assertClean(t)
	})
}

func TestBrowserDiscoveryPreservesConfiguredRemoteHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			t.Errorf("unexpected discovery path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"webSocketDebuggerUrl":"ws://localhost:9222/devtools/browser/example"}`)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, address := range []string{server.URL, strings.TrimPrefix(server.URL, "http://"), strings.Replace(server.URL, "http://", "ws://", 1)} {
		got, err := resolveBrowserURL(ctx, address)
		want := strings.Replace(server.URL, "http://", "ws://", 1) + "/devtools/browser/example"
		if err != nil || got != want {
			t.Fatalf("endpoint %s resolved to %s, %v; want %s", address, got, err, want)
		}
	}
}

func TestBrowserQueueHonorsFeedContext(t *testing.T) {
	for range cap(browserSlots) {
		browserSlots <- struct{}{}
	}
	defer func() {
		for range cap(browserSlots) {
			<-browserSlots
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	got := ParsePageByBrowserContext(ctx, define.JavaScriptConfig{URL: "https://example.org", DisableCache: true, Timeout: 10}, "unreachable", "", false)
	if ctx.Err() != context.DeadlineExceeded || got.Code == define.ERROR_CODE_NULL {
		t.Fatalf("feed queue timeout lost: %#v", got)
	}
}
