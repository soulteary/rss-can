package access

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestWardenChecksStableIdentityAndStatus(t *testing.T) {
	for _, tc := range []struct {
		status  int
		body    string
		allowed bool
	}{{200, `{"user_id":"alice","status":"active"}`, true}, {200, `{"user_id":"other","status":"active"}`, false}, {200, `{"user_id":"alice","status":"inactive"}`, false}, {200, `{"user_id":"alice"}`, false}, {200, `{}`, false}, {404, `{}`, false}, {500, `{}`, false}, {200, `not json`, false}} {
		t.Run(fmt.Sprintf("%d-%s", tc.status, tc.body), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/user" || r.URL.Query().Get("user_id") != "alice" || r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("incorrect Warden contract")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			err := (WardenChecker{URL: srv.URL, APIKey: "secret"}).Active(context.Background(), "alice")
			if (err == nil) != tc.allowed {
				t.Fatalf("err %v allowed %v", err, tc.allowed)
			}
		})
	}
}

func TestWardenDoesNotFollowRedirectOrIgnoreCancellation(t *testing.T) {
	visited := false
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visited = true }))
	defer dst.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dst.URL, 302) }))
	defer src.Close()
	w := WardenChecker{URL: src.URL, APIKey: "secret"}
	if err := w.Active(context.Background(), "alice"); err == nil {
		t.Fatal("redirect allowed")
	}
	if visited {
		t.Fatal("credential redirect followed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.Active(ctx, "alice"); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestConfigRequiresExplicitTrust(t *testing.T) {
	c := Config{StorePath: "/private/access.json", PublicOrigin: "https://rss.example", TrustedProxies: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32")}, WardenURL: "https://warden.example", WardenAPIKey: "key"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if !c.Trusted("192.0.2.1:123") || c.Trusted("192.0.2.2:123") || c.Trusted("garbage") {
		t.Fatal("bad proxy trust")
	}
	for _, origin := range []string{"http://rss.example", "https://rss.example/path", "https://user@rss.example", "https://rss.example?x=1"} {
		bad := c
		bad.PublicOrigin = origin
		if bad.Validate() == nil {
			t.Fatalf("accepted origin %s", origin)
		}
	}
	bad := c
	bad.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}
	if bad.Validate() == nil {
		t.Fatal("trust all accepted")
	}
	t.Setenv("RSS_ACCESS_MODE", "invalid")
	if _, err := FromEnv(); err == nil {
		t.Fatal("invalid mode")
	}
	t.Setenv("RSS_ACCESS_MODE", "standalone")
	if cfg, err := FromEnv(); cfg != nil || err != nil {
		t.Fatal("legacy mode changed")
	}
}
