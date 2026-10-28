package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func TestManagementAccess(t *testing.T) {
	for _, tc := range []struct {
		name, token, remote, supplied, forwarded, site, origin string
		want                                                   int
	}{
		{name: "remote denied", remote: "192.0.2.1:1234", want: 403},
		{name: "spoofed forwarded loopback denied", remote: "192.0.2.1:1234", forwarded: "127.0.0.1", want: 403},
		{name: "loopback proxy denied", remote: "127.0.0.1:1234", forwarded: "192.0.2.1", want: 403},
		{name: "local allowed", remote: "127.0.0.1:1234", want: 200},
		{name: "IPv6 loopback allowed", remote: "[::1]:1234", want: 200},
		{name: "token required locally", token: "secret", remote: "127.0.0.1:1234", want: 401},
		{name: "wrong token denied", token: "secret", remote: "192.0.2.1:1234", supplied: "Bearer bad", want: 401},
		{name: "authorized remote", token: "secret", remote: "192.0.2.1:1234", supplied: "Bearer secret", want: 200},
		{name: "cross site denied", remote: "127.0.0.1:1234", site: "cross-site", want: 403},
		{name: "foreign origin denied", remote: "127.0.0.1:1234", origin: "https://attacker.invalid", want: 403},
		{name: "opaque inspector origin denied", remote: "127.0.0.1:1234", origin: "null", want: 403},
		{name: "same origin allowed", remote: "127.0.0.1:1234", origin: "http://localhost", site: "same-origin", want: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := adminAccessStatus(adminRequest{
				token:         tc.token,
				site:          tc.site,
				origin:        tc.origin,
				host:          "localhost",
				authorization: tc.supplied,
				xForwardedFor: tc.forwarded,
				remoteAddr:    tc.remote,
			})
			if got == 0 {
				got = http.StatusOK
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func testResponse(t *testing.T, app *fiber.App, req *http.Request) (int, string) {
	t.Helper()
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}

func installTestRule(t *testing.T, upstream string) (string, string) {
	t.Helper()
	logger.Initialize()
	previous := define.RULES_DIRECTORY
	root := t.TempDir()
	dir := filepath.Join(root, "example")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "feed.js")
	body := fmt.Sprintf(`potted.SetScope("article").SetConfig({Name:"Before",URL:%q,Mode:"ssr",Title:"h2",Link:"a",DisableCache:true});`, upstream)
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	define.RULES_DIRECTORY = root
	rule.InitRules()
	t.Cleanup(func() { define.RULES_DIRECTORY = previous; rule.InitRules() })
	return file, body
}

func TestFeedReturnsBadGatewayForUpstreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer upstream.Close()
	installTestRule(t, upstream.URL)
	route := fiber.New()
	route.Get("/feed/:id/:type/", apiRSS())
	status, body := testResponse(t, route, httptest.NewRequest("GET", "/feed/example/rss/", nil))
	if status != http.StatusBadGateway || !strings.Contains(body, "get website data failed") {
		t.Fatalf("response: %d %s", status, body)
	}
}

func TestListReflectsReload(t *testing.T) {
	file, body := installTestRule(t, "https://example.org")
	previous := define.DEBUG_MODE
	define.DEBUG_MODE = false
	defer func() { define.DEBUG_MODE = previous }()
	route := fiber.New()
	route.Get("/feed", listPage())
	get := func() string {
		_, body := testResponse(t, route, httptest.NewRequest("GET", "/feed", nil))
		return body
	}
	if !strings.Contains(get(), "Before") {
		t.Fatal("initial rule missing")
	}
	if err := os.WriteFile(file, []byte(strings.ReplaceAll(body, "Before", "After")), 0600); err != nil {
		t.Fatal(err)
	}
	rule.InitRules()
	if html := get(); !strings.Contains(html, "After") || strings.Contains(html, "Before") {
		t.Fatal("list did not reflect reloaded rule")
	}
}

func TestInspectorRejectsPrivateTargetBeforeBrowserConnection(t *testing.T) {
	logger.Initialize()
	route := fiber.New()
	route.Get("/inspector/:url/", inspectorProxy())
	for _, target := range []string{"http://127.0.0.1:6379", "file:///etc/passwd", "http://169.254.169.254"} {
		status, _ := testResponse(t, route, httptest.NewRequest("GET", "/inspector/"+fn.Base64Encode(target)+"/", nil))
		if status != http.StatusBadRequest {
			t.Errorf("target %s: %d", target, status)
		}
	}
}

func TestManagementRejectsRebindingHost(t *testing.T) {
	t.Setenv("RSS_ADMIN_TOKEN", "")
	route := fiber.New()
	route.Get("/config", adminOnly(), func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	req := httptest.NewRequest("GET", "http://attacker.invalid/config", nil)
	status, _ := testResponse(t, route, req)
	if status != http.StatusForbidden {
		t.Fatal("non-local Host accepted without a token")
	}
}

func TestInspectorHomeUsesEmbeddedAssets(t *testing.T) {
	route := fiber.New()
	route.Get("/inspector", inspectorHome())
	status, body := testResponse(t, route, httptest.NewRequest("GET", "/inspector", nil))
	if status != http.StatusOK || !strings.Contains(body, "<html") {
		t.Fatal("inspector template not embedded")
	}
}

func TestFiberUtilityRoutesAndEmbeddedAssets(t *testing.T) {
	route := fiber.New()
	route.Get("/_/health/", apiHealth())
	route.Get("/_/1x1.gif", PixelImage())
	route.Use("/assets", static.New("/", static.Config{FS: ServerAssets()}))

	for _, test := range []struct {
		path, contentType, body string
	}{
		{path: "/_/health/", contentType: "application/json", body: `"pong"`},
		{path: "/_/1x1.gif", contentType: "image/gif", body: "GIF89a"},
		{path: "/assets/css/tailwind.min.css", contentType: "text/css", body: "--tw-"},
	} {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		response, err := route.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Errorf("%s: got status %d", test.path, response.StatusCode)
		}
		if !strings.HasPrefix(response.Header.Get("Content-Type"), test.contentType) {
			t.Errorf("%s: got content type %q", test.path, response.Header.Get("Content-Type"))
		}
		if !strings.Contains(string(data), test.body) {
			t.Errorf("%s: expected body to contain %q", test.path, test.body)
		}
	}
}

func TestFeedRejectsUnsupportedTypeBeforeRuleExecution(t *testing.T) {
	installTestRule(t, "http://unused.invalid")
	// A valid format would require Deno; an invalid format must fail before it.
	jssdk.CloseRuntime()
	t.Setenv("RSS_CAN_DENO", filepath.Join(t.TempDir(), "missing-deno"))
	route := fiber.New()
	route.Get("/feed/:id/:type/", apiRSS())
	status, body := testResponse(t, route, httptest.NewRequest("GET", "/feed/example/invalid/", nil))
	if status != http.StatusBadRequest || !strings.Contains(body, "unsupported feed type") {
		t.Fatalf("response: %d %s", status, body)
	}
}

func TestFeedInvalidRuleIsServerError(t *testing.T) {
	file, _ := installTestRule(t, "http://unused.invalid")
	if err := os.WriteFile(file, []byte(`potted.SetConfig({IdByRegexp:"["});`), 0600); err != nil {
		t.Fatal(err)
	}
	rule.InitRules()
	route := fiber.New()
	route.Get("/feed/:id/:type/", apiRSS())
	status, body := testResponse(t, route, httptest.NewRequest("GET", "/feed/example/rss/", nil))
	if status != http.StatusInternalServerError || !strings.Contains(body, "parse config failed") {
		t.Fatalf("response: %d %s", status, body)
	}
}

func TestFeedDeadlineReturnsGatewayTimeout(t *testing.T) {
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(upstreamCanceled)
	}))
	defer upstream.Close()
	installTestRule(t, upstream.URL)
	if err := jssdk.InitializeRuntime(); err != nil {
		t.Fatal(err)
	}
	route := fiber.New()
	route.Use(func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 150*time.Millisecond)
		defer cancel()
		c.SetContext(ctx)
		return c.Next()
	})
	route.Get("/feed/:id/:type/", apiRSS())
	status, body := testResponse(t, route, httptest.NewRequest("GET", "/feed/example/rss/", nil))
	if status != http.StatusGatewayTimeout || !strings.Contains(body, "timed out") {
		t.Fatalf("response: %d %s", status, body)
	}
	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("timed out upstream left running")
	}
}

func TestFeedGenerationFailureIsServerError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate runtime loss between rule evaluation and date conversion.
		jssdk.CloseRuntime()
		fmt.Fprint(w, `<article><h2>title</h2><a href="/post">post</a><span>2026-10-28</span></article>`)
	}))
	defer upstream.Close()
	file, body := installTestRule(t, upstream.URL)
	body = strings.Replace(body, `Title:"h2"`, `DateTime:"span",Title:"h2"`, 1)
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	rule.InitRules()
	if err := jssdk.InitializeRuntime(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RSS_CAN_DENO", filepath.Join(t.TempDir(), "missing-deno"))
	route := fiber.New()
	route.Get("/feed/:id/:type/", apiRSS())
	status, response := testResponse(t, route, httptest.NewRequest("GET", "/feed/example/rss/", nil))
	if status != http.StatusInternalServerError || !strings.Contains(response, "generate feed failed") {
		t.Fatalf("response: %d %s", status, response)
	}
}

func TestFeedFormatsStillSucceed(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<article><h2>title</h2><a href="/post">post</a></article>`)
	}))
	defer upstream.Close()
	installTestRule(t, upstream.URL)
	route := fiber.New()
	route.Get("/feed/:id/:type/", apiRSS())
	for _, format := range []string{"rss", "atom", "json", "RSS"} {
		response, err := route.Test(httptest.NewRequest("GET", "/feed/example/"+format+"/", nil))
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		mime, _ := feedMIME(strings.ToLower(format))
		if err != nil || response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), mime) || !strings.Contains(string(body), "title") {
			t.Fatalf("%s: %d %s %v", format, response.StatusCode, body, err)
		}
	}
}

func TestFeedConfiguredServerTimeout(t *testing.T) {
	previous := define.SERVER_TIMEOUT
	define.SERVER_TIMEOUT = 1
	defer func() { define.SERVER_TIMEOUT = previous }()
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(canceled)
	}))
	defer upstream.Close()
	installTestRule(t, upstream.URL)
	route := fiber.New()
	route.Get("/feed/:id/:type/", apiRSS())
	began := time.Now()
	response, err := route.Test(httptest.NewRequest("GET", "/feed/example/rss/", nil), fiber.TestConfig{Timeout: 3 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("configured timeout returned %d", response.StatusCode)
	}
	if elapsed := time.Since(began); elapsed > 2500*time.Millisecond {
		t.Fatalf("configured timeout ignored: %v", elapsed)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("upstream kept running after configured timeout")
	}
}
