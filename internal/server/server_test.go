package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func TestManagementAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
			t.Setenv("RSS_ADMIN_TOKEN", tc.token)
			route := gin.New()
			route.GET("/config", adminOnly(), func(c *gin.Context) { c.Status(200) })
			req := httptest.NewRequest("GET", "http://localhost/config", nil)
			req.RemoteAddr = tc.remote
			req.Header.Set("Authorization", tc.supplied)
			req.Header.Set("X-Forwarded-For", tc.forwarded)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			route.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
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
	route := gin.New()
	route.GET("/feed/:id/:type/", apiRSS())
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, httptest.NewRequest("GET", "/feed/example/rss/", nil))
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "get website data failed") {
		t.Fatalf("response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestListReflectsReload(t *testing.T) {
	file, body := installTestRule(t, "https://example.org")
	previous := define.DEBUG_MODE
	define.DEBUG_MODE = false
	defer func() { define.DEBUG_MODE = previous }()
	route := gin.New()
	route.GET("/feed", listPage())
	get := func() string {
		rec := httptest.NewRecorder()
		route.ServeHTTP(rec, httptest.NewRequest("GET", "/feed", nil))
		return rec.Body.String()
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
	route := gin.New()
	route.GET("/inspector/:url/", inspectorProxy())
	for _, target := range []string{"http://127.0.0.1:6379", "file:///etc/passwd", "http://169.254.169.254"} {
		rec := httptest.NewRecorder()
		route.ServeHTTP(rec, httptest.NewRequest("GET", "/inspector/"+fn.Base64Encode(target)+"/", nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("target %s: %d", target, rec.Code)
		}
	}
}

func TestManagementRejectsRebindingHost(t *testing.T) {
	t.Setenv("RSS_ADMIN_TOKEN", "")
	route := gin.New()
	route.GET("/config", adminOnly(), func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest("GET", "http://attacker.invalid/config", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatal("non-local Host accepted without a token")
	}
}

func TestInspectorHomeUsesEmbeddedAssets(t *testing.T) {
	route := gin.New()
	route.GET("/inspector", inspectorHome())
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, httptest.NewRequest("GET", "/inspector", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<html") {
		t.Fatal("inspector template not embedded")
	}
}
