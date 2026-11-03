package application

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/network"
)

func TestTeamModeCannotSelectTrustedFetchPolicy(t *testing.T) {
	t.Setenv("RSS_ACCESS_MODE", "team")
	t.Setenv("RSS_FETCH_POLICY", "trusted")
	config, err := ConfigFromGlobals()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Network.Policy.PublicOnly() {
		t.Fatal("team mode permitted trusted fetches")
	}
}
func TestInvalidFetchPolicyRejectedAtStartup(t *testing.T) {
	t.Setenv("RSS_FETCH_POLICY", "pubic")
	if _, err := ConfigFromGlobals(); err == nil {
		t.Fatal("misspelled policy accepted")
	}
}
func TestPublicFetchProxyRejectedBeforeRuntimeStartup(t *testing.T) {
	config, err := ConfigFromGlobals()
	if err != nil {
		t.Fatal(err)
	}
	config.Network.Policy = network.PublicPolicy()
	config.Network.Proxy = "http://127.0.0.1:8080"
	if _, err := New(t.Context(), config); err == nil || !strings.Contains(err.Error(), "does not allow a proxy") {
		t.Fatalf("unexpected startup error %v", err)
	}
}

func TestPublicSSRUsesPolicyWithoutInjectedBrowserOverride(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<article><h2>Allowed headline</h2><a href="/post">Read</a></article>`)
	}))
	defer upstream.Close()
	config := testConfig(t, "public-ssr", "http://source.example")
	config.Network.Policy = network.PublicPolicy()
	config.Network.LookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	config.Network.DialContext = func(ctx context.Context, networkName, address string) (net.Conn, error) {
		if address != "8.8.8.8:80" {
			t.Errorf("unvalidated dial: %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, networkName, strings.TrimPrefix(upstream.URL, "http://"))
	}
	app, err := New(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	status, body := response(t, app, "/public-ssr/site/rss/")
	if status != 200 || !strings.Contains(body, "Allowed headline") {
		t.Fatalf("public SSR failed: %d %s", status, body)
	}
}
