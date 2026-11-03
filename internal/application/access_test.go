package application

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/access"
)

func teamConfig(t *testing.T) Config {
	t.Helper()
	cfg := testConfig(t, "team", "https://example.org")
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.Server.AdminToken = strings.Repeat("a", 32)
	cfg.Server.Access = &access.Config{StorePath: filepath.Join(dir, "access.json"), PublicOrigin: "https://rss.example", TrustedProxies: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/32")}, WardenURL: "https://warden.example", WardenAPIKey: "test-key"}
	return cfg
}

func TestTeamApplicationOwnsAuthorizationLifecycle(t *testing.T) {
	cfg := teamConfig(t)
	warden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("user_id") != "alice" {
			t.Error("wrong subject")
		}
		_, _ = w.Write([]byte(`{"user_id":"alice","status":"active"}`))
	}))
	defer warden.Close()
	cfg.Server.Access.WardenURL = warden.URL
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err = app.access.PutSource(access.Source{ID: "site", Owner: "alice"}); err != nil {
		t.Fatal(err)
	}
	if status, _ := response(t, app, "/team/site/rss/"); status != 401 {
		t.Fatal("anonymous legacy URL exposed")
	}
	req := httptest.NewRequest("GET", "/api/sources", nil)
	req.Header.Set("X-Auth-User", "alice")
	res, err := app.Routes().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("identity not wired: %d", res.StatusCode)
	}
	cfg.Server.Access.TrustedProxies[0] = netip.MustParsePrefix("192.0.2.1/32")
	if !app.config.Server.Access.Trusted("0.0.0.0:0") {
		t.Fatal("captured trust config was mutable")
	}
	if err = app.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := access.Open(cfg.Server.Access.StorePath)
	if err != nil {
		t.Fatal("close did not release access lock", err)
	}
	defer reopened.Close()
	if _, ok := reopened.CanRead("alice", "site"); !ok {
		t.Fatal("permissions lost on application close")
	}
}

func TestTeamApplicationRejectsInvalidConfigurationAndReleasesFailedStartup(t *testing.T) {
	cfg := teamConfig(t)
	cfg.Server.AdminToken = "short"
	if app, err := New(t.Context(), cfg); err == nil || app != nil {
		t.Fatal("weak admin token accepted")
	}
	cfg.Server.AdminToken = strings.Repeat("a", 32)
	cfg.Server.Timeout = 0
	if app, err := New(t.Context(), cfg); err == nil || app != nil {
		t.Fatal("bad timeout accepted")
	}
	s, err := access.Open(cfg.Server.Access.StorePath)
	if err != nil {
		t.Fatal("failed startup leaked file lock", err)
	}
	_ = s.Close()
	cfg.Server.Access.PublicOrigin = "http://insecure.example"
	if app, err := New(t.Context(), cfg); err == nil || app != nil {
		t.Fatal("insecure origin accepted")
	}
	t.Setenv("RSS_ACCESS_MODE", "team")
	t.Setenv("RSS_TRUSTED_PROXIES", "")
	if _, err := ConfigFromGlobals(); err == nil {
		t.Fatal("invalid env accepted")
	}
}
