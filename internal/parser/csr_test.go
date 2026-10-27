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

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
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
	cached := ParsePageByGoRod(config, "unreachable-browser", "", false)
	if cached.Code != 0 || len(cached.Body) != 1 || cached.Body[0].Title != "Cached title" {
		t.Fatalf("JSON cache parsed as HTML: %#v", cached)
	}
	if err := cacher.Set(browserCacheKey(config, true), `<article><h2>Mixed title</h2><a href="/item">Read</a></article>`); err != nil {
		t.Fatal(err)
	}
	mixed := ParsePageByGoRod(config, "unreachable-browser", "", true)
	if mixed.Code != 0 || len(mixed.Body) != 1 || mixed.Body[0].Title != "Mixed title" {
		t.Fatalf("HTML cache not parsed correctly: %#v", mixed)
	}
	// An old plain-URL SSR cache entry must not replace the extracted CSR JSON.
	if err := cacher.Set(config.URL, "wrong representation"); err != nil {
		t.Fatal(err)
	}
	again := ParsePageByGoRod(config, "unreachable-browser", "", false)
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
	if err := withRodPage(ctx2, "unreachable", "", "", "", false, func(*rod.Page) { t.Fatal("full queue admitted browser work") }); err == nil {
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
	if err := withRodPage(ctx, "ws://"+listener.Addr().String()+"/devtools/browser/stalled", "", "", "", false, func(*rod.Page) { t.Error("stalled handshake succeeded") }); err == nil {
		t.Fatal("handshake did not time out")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handshake socket leaked")
	}
}

func TestBrowserLifecycleAndLiveCache(t *testing.T) {
	binary, found := launcher.LookPath()
	if configured := os.Getenv("RSS_TEST_CHROME"); configured != "" {
		binary = configured
		found = true
	}
	if !found {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("Chrome/Chromium required for browser regression tests")
		}
		t.Skip("Chrome/Chromium not installed; set RSS_TEST_CHROME to enable browser integration tests")
	}
	memoryCacheForTest(t)
	l := launcher.New().Bin(binary).Leakless(false).Headless(true).UserDataDir(t.TempDir())
	address, err := l.Launch()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Cleanup()
	defer l.Kill()
	controller := rod.New().ControlURL(address)
	if err := controller.Connect(); err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	assertClean := func(t *testing.T) {
		t.Helper()
		contexts, err := (proto.TargetGetBrowserContexts{}).Call(controller)
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
	t.Run("dispose context on lost connection", func(t *testing.T) {
		// Simulate losing the CDP connection before explicit cleanup can run.
		ws := &cdp.WebSocket{}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := ws.Connect(ctx, address, nil); err != nil {
			t.Fatal(err)
		}
		defer ws.Close()
		detached := rod.New().Client(cdp.New().Start(ws)).Context(ctx)
		if err := detached.Connect(); err != nil {
			t.Fatal(err)
		}
		created, err := (proto.TargetCreateBrowserContext{DisposeOnDetach: true}).Call(detached)
		if err != nil {
			t.Fatal(err)
		}
		if created.BrowserContextID == "" {
			t.Fatal("missing browser context")
		}
		if err := ws.Close(); err != nil {
			t.Fatal(err)
		}
		// Disposal is asynchronous when the connection disappears.
		for {
			contexts, err := (proto.TargetGetBrowserContexts{}).Call(controller)
			if err != nil {
				t.Fatal(err)
			}
			if len(contexts.BrowserContextIDs) == 0 {
				break
			}
			if ctx.Err() != nil {
				t.Fatal("context survived lost CDP connection")
			}
			time.Sleep(10 * time.Millisecond)
		}
		assertClean(t)
	})
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
			first := ParsePageByGoRod(config, address, "", mixed)
			if first.Code != 0 || len(first.Body) != 1 || first.Body[0].Title != "Live title" {
				t.Fatalf("live parse: %#v", first)
			}
			count := requests.Load()
			cached := ParsePageByGoRod(config, "ws://127.0.0.1:1", "", mixed)
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
		err := withRodPage(ctx, address, "", upstream.URL, "", false, func(page *rod.Page) {
			page.MustNavigate(upstream.URL).MustWaitLoad()
			page.MustEval(`() => { window.originalDollar = window.$ = {pageOwned: true}; window.originalJQuery = window.jQuery = {pageOwned: true}; }`)
			sdkRule := append(append([]byte{}, rule...), []byte(`
    if (typeof jQuery !== "function" || jQuery.fn.jquery !== "4.0.0" || $ !== jQuery || POTTED.fn.$ !== $) {
     throw new Error("private jQuery aliases are missing or inconsistent");
    }
   `)...)
			code := jssdk.GenerateCSRInjectParser(sdkRule)
			for i := 0; i < 2; i++ {
				result := parseCSRItems(fmt.Sprint(page.MustEval(code)))
				if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Title != "Live title" {
					panic(fmt.Errorf("SDK result: %#v", result))
				}
				preserved := page.MustEval(`() => window.$ === window.originalDollar && window.jQuery === window.originalJQuery`).Bool()
				if !preserved {
					panic("SDK replaced the page's jQuery globals")
				}
			}
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
		result := ParsePageByGoRod(broken, address, "", false)
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
		result := ParsePageByGoRod(broken, address, "", false)
		if result.Code == 0 {
			t.Fatal("infinite browser script treated as success")
		}
		assertClean(t)
	})
	t.Run("cleanup after panic", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := withRodPage(ctx, address, "", upstream.URL, "", false, func(*rod.Page) { panic("test failure") }); err == nil {
			t.Fatal("panic swallowed")
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

func TestBrowserWebSocketDefaultPorts(t *testing.T) {
	for _, tc := range []struct {
		address string
		secure  bool
		want    string
	}{
		{"example.org", false, "example.org:80"},
		{"example.org", true, "example.org:443"},
		{"[::1]", true, "[::1]:443"},
		{"example.org:9222", true, "example.org:9222"},
		{"[::1]:9222", false, "[::1]:9222"},
	} {
		if got := browserDialAddress(tc.address, tc.secure); got != tc.want {
			t.Errorf("address %s: got %s, want %s", tc.address, got, tc.want)
		}
	}
}
