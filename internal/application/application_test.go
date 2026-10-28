package application

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/server"
)

func testConfig(t *testing.T, name, target string) Config {
	t.Helper()
	logger.Initialize()
	binary := os.Getenv("RSS_CAN_DENO")
	if binary == "" {
		var err error
		binary, err = exec.LookPath("deno")
		if err != nil {
			t.Skip("Deno is required")
		}
	}
	root := t.TempDir()
	group := filepath.Join(root, "site")
	if err := os.Mkdir(group, 0700); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`potted.SetScope("article").SetConfig({Name:%q,URL:%q,Mode:"ssr",Title:"h2",Link:"a"});`, name, target)
	if err := os.WriteFile(filepath.Join(group, "feed.js"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return Config{Server: server.Options{Host: "127.0.0.1", Port: 0, FeedPath: "/" + name, RuleDirectory: root, Timeout: 3 * time.Second, ShutdownTimeout: 4 * time.Second, Headless: "127.0.0.1:9222"}, Cache: cacher.Config{MemoryEnabled: true, MemoryTTL: time.Minute}, JavaScript: jssdk.RuntimeConfig{Binary: binary, Workers: 1, Timeout: time.Second}, Network: network.Config{RequestTimeout: time.Second, DefaultTTL: time.Minute, UserAgent: name}, BrowserTimeout: time.Second}
}
func response(t *testing.T, app *App, path string) (int, string) {
	t.Helper()
	res, err := app.Routes().Test(httptest.NewRequest("GET", path, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body)
}
func TestApplicationInstancesKeepRulesCacheAndRuntimeIndependent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<article><h2>%s</h2><a href="/post">read</a></article>`, r.UserAgent())
	}))
	defer upstream.Close()
	first, err := New(t.Context(), testConfig(t, "first", upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := New(t.Context(), testConfig(t, "second", upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	// Production instances must not reread legacy mutable options.
	oldUA, oldProxy, oldDir := define.USER_AGENT, define.PROXY_SERVER, define.RULES_DIRECTORY
	define.USER_AGENT = "global"
	define.PROXY_SERVER = "http://127.0.0.1:1"
	define.RULES_DIRECTORY = "missing"
	defer func() { define.USER_AGENT, define.PROXY_SERVER, define.RULES_DIRECTORY = oldUA, oldProxy, oldDir }()
	for _, tc := range []struct {
		app  *App
		name string
	}{{first, "first"}, {second, "second"}} {
		status, body := response(t, tc.app, "/"+tc.name+"/site/rss/")
		if status != 200 || (!strings.Contains(body, "<title>"+tc.name+"</title>") || strings.Contains(body, "global")) {
			t.Fatalf("%s: %d %s", tc.name, status, body)
		}
		status, body = response(t, tc.app, "/"+tc.name)
		if status != 200 || !strings.Contains(body, tc.name) || strings.Contains(body, "missing") {
			t.Fatalf("list %s: %d %s", tc.name, status, body)
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.runtime.Check(t.Context()); err != nil {
		t.Fatalf("closing first damaged second: %v", err)
	}
	if status, body := response(t, second, "/second/site/rss/"); status != 200 || !strings.Contains(body, "second") {
		t.Fatalf("second after close: %d %s", status, body)
	}
}

func TestApplicationCloseCancelsAndJoinsActiveRequest(t *testing.T) {
	entered, canceled := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(canceled) }))
	defer upstream.Close()
	app, err := New(t.Context(), testConfig(t, "closing", upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	result := make(chan int, 1)
	go func() {
		res, err := app.Routes().Test(httptest.NewRequest("GET", "/closing/site/rss/", nil))
		if err != nil {
			result <- 0
			return
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		result <- res.StatusCode
	}()
	<-entered
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case status := <-result:
		if status != 504 {
			t.Fatalf("canceled request status %d", status)
		}
	case <-time.After(time.Second):
		t.Fatal("handler was not joined")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("upstream left running")
	}
	if err := app.runtime.Check(context.Background()); err == nil {
		t.Fatal("runtime restarted after close")
	}
	if status, _ := response(t, app, "/_/health/"); status != 503 {
		t.Fatalf("closed app admitted request: %d", status)
	}
}

func TestApplicationStartupFailureAndListenerFailure(t *testing.T) {
	cfg := testConfig(t, "startup", "https://example.org")
	good, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	good.Close()
	cfg.Server.RuleDirectory = filepath.Join(t.TempDir(), "missing")
	if app, err := New(t.Context(), cfg); err == nil || app != nil {
		t.Fatal("invalid startup published an app")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg = testConfig(t, "listen", "https://example.org")
	cfg.Server.Port = listener.Addr().(*net.TCPAddr).Port
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(t.Context()); err == nil {
		t.Fatal("listener error lost")
	}
	if err := app.runtime.Check(context.Background()); err == nil {
		t.Fatal("listener failure did not close runtime")
	}
}

func TestApplicationConfigCaptureAndValidation(t *testing.T) {
	t.Setenv("RSS_JS_WORKERS", "2")
	t.Setenv("RSS_CONFIG_CACHE_TTL", "9000")
	cfg, err := ConfigFromGlobals()
	if err != nil || cfg.JavaScript.Workers != 2 || cfg.ConfigCacheTTL != time.Hour {
		t.Fatalf("%+v %v", cfg, err)
	}
	t.Setenv("RSS_JS_WORKERS", "bad")
	if _, err := ConfigFromGlobals(); err == nil {
		t.Fatal("invalid worker configuration")
	}
	if _, err := New(t.Context(), Config{}); err == nil {
		t.Fatal("invalid timeout accepted")
	}
	cfg.Server.FeedPath = ""
	if _, err := New(t.Context(), cfg); err == nil {
		t.Fatal("empty path accepted")
	}
}

func TestApplicationRunStopsOnOwnedClose(t *testing.T) {
	cfg := testConfig(t, "ownedclose", "https://example.org")
	reserve, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.Port = reserve.Addr().(*net.TCPAddr).Port
	reserve.Close()
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	done := make(chan error, 1)
	go func() { done <- app.Run(context.Background()) }()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/_/health/", cfg.Server.Port)
	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for {
		res, err := client.Get(endpoint)
		if err == nil {
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			break
		}
		select {
		case err := <-done:
			t.Fatalf("server failed to start: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("listener failed to become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("owned close left listener running")
	}
	if res, err := client.Get(endpoint); err == nil {
		res.Body.Close()
		t.Fatal("closed app still listening")
	}
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("closed app started again")
	}
}
