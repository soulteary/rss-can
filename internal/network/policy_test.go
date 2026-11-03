package network

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
)

func TestPublicFetchPolicyURLsAndOverrides(t *testing.T) {
	p := PublicPolicy()
	for _, raw := range []string{"http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/", "http://100.64.0.1/", "http://192.0.2.1/", "http://[::ffff:127.0.0.1]/", "http://[2002:a00:1::]/", "http://[fec0::1]/", "http://example.com:8080/", "file:///etc/passwd", "https://user:pass@example.com/"} {
		if p.ValidateURL(raw) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"http://example.com/", "https://example.com:443/", "https://8.8.8.8/"} {
		if err := p.ValidateURL(raw); err != nil {
			t.Errorf("rejected %s: %v", raw, err)
		}
	}
	for _, input := range []struct{ proxy, headless, mode string }{{"http://proxy:8080", "", "ssr"}, {"", "browser:3000", "ssr"}, {"", "", "csr"}, {"", "", "mix"}} {
		if p.ValidateRule("https://example.com", input.proxy, input.headless, input.mode) == nil {
			t.Fatalf("accepted override: %+v", input)
		}
	}
	if _, err := ParseFetchPolicy("typo"); err == nil {
		t.Fatal("invalid policy accepted")
	}
}
func TestPolicyPinsResolvedAddressesAndRejectsMixedDNS(t *testing.T) {
	calls := 0
	lookup := func(context.Context, string, string) ([]netip.Addr, error) {
		calls++
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	var address string
	dial := func(_ context.Context, _ string, addr string) (net.Conn, error) {
		address = addr
		return nil, errors.New("test stop")
	}
	_, _ = policyDialContext(t.Context(), "tcp", "example.com:443", lookup, dial)
	if calls != 1 || address != "8.8.8.8:443" {
		t.Fatalf("lookup=%d address=%s", calls, address)
	}
	lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, nil
	}
	address = ""
	if _, err := policyDialContext(t.Context(), "tcp", "example.com:443", lookup, dial); !errors.Is(err, ErrFetchPolicy) || address != "" {
		t.Fatal("mixed public/private DNS dialed")
	}
}
func TestOriginJarDoesNotShareCredentialsAcrossSchemeOrPort(t *testing.T) {
	original, _ := url.Parse("https://example.com:443/a")
	jar := newOriginJar(original)
	jar.SetCookies(original, []*http.Cookie{{Name: "secret", Value: "value", Path: "/"}})
	for _, raw := range []string{"http://example.com/a", "https://example.com:8443/a", "https://child.example.com/a"} {
		u, _ := url.Parse(raw)
		if len(jar.Cookies(u)) != 0 {
			t.Fatalf("credentials leaked to %s", raw)
		}
	}
	same, _ := url.Parse("https://example.com/b")
	if len(jar.Cookies(same)) != 1 {
		t.Fatal("same-origin cookie absent")
	}
}
func TestFetchPolicyRejectsRedirectAndProxy(t *testing.T) {
	ctx := WithFetchPolicy(t.Context(), PublicPolicy())
	client, tr, err := policyHTTPClient(ctx, time.Second, "")
	if err != nil {
		t.Fatal(err)
	}
	defer tr.CloseIdleConnections()
	req, _ := http.NewRequest("GET", "http://127.0.0.1/", nil)
	if err := client.CheckRedirect(req, []*http.Request{{}}); !errors.Is(err, ErrFetchPolicy) {
		t.Fatalf("redirect accepted: %v", err)
	}
	if _, _, err := policyHTTPClient(ctx, time.Second, "http://127.0.0.1:9999"); !errors.Is(err, ErrFetchPolicy) {
		t.Fatal("public proxy accepted")
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9999")
	if tr.Proxy != nil {
		t.Fatal("public client inherits environment proxy")
	}
}
func TestTaskBudgetCountsRedirectsAndBodies(t *testing.T) {
	ctx := WithFetchBudget(t.Context())
	b := taskBudget(ctx)
	if taskBudget(WithFetchBudget(ctx)) != b {
		t.Fatal("budget replaced")
	}
	for i := 0; i < MaxTaskRequests; i++ {
		if err := b.request(); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(b.request(), ErrFetchBudget) {
		t.Fatal("unlimited requests")
	}
	b.bytes = MaxTaskBodyBytes - 1
	reader := &policyBody{ReadCloser: io.NopCloser(strings.NewReader("ab")), budget: b}
	if _, err := io.ReadAll(reader); !errors.Is(err, ErrFetchBudget) {
		t.Fatal("unlimited bytes")
	}
}
func TestPolicyScopeAndCachedPrivateTarget(t *testing.T) {
	trusted := WithFetchPolicy(t.Context(), FetchPolicy{Mode: FetchTrusted})
	public := WithFetchPolicy(t.Context(), PublicPolicy())
	if policyDocumentKey(trusted, "key") == policyDocumentKey(public, "key") {
		t.Fatal("policies share cache")
	}
	got := GetRemoteDocumentContext(public, "http://127.0.0.1/", "", 60, false)
	if got.Code == 0 {
		t.Fatal("private cache target accepted")
	}
}
func TestTrustedHTTPRedirectBudgetAndCookieBoundary(t *testing.T) {
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if r.Header.Get("Cookie") != "" {
			t.Error("cookie leaked across port")
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	ctx := WithFetchBudget(t.Context())
	code, _, res := HttpGetContext(ctx, source.URL, "", RequestOptions{Cookies: "secret=value", CookieURL: source.URL})
	if code != 0 {
		t.Fatalf("fetch failed: %d", code)
	}
	defer res.Body.Close()
	_, _ = io.ReadAll(res.Body)
	if !reached || taskBudget(ctx).requests != 2 {
		t.Fatalf("reached=%v requests=%d", reached, taskBudget(ctx).requests)
	}
}

func TestPublicPolicyRejectsForbiddenOptionsBeforeCacheHit(t *testing.T) {
	cache := cacher.New(cacher.Config{MemoryEnabled: true, MemoryTTL: time.Minute})
	defer cache.Close()
	ctx := cacher.WithCache(WithFetchPolicy(t.Context(), PublicPolicy()), cache)
	for _, tc := range []struct {
		target  string
		options RequestOptions
	}{
		{target: "http://127.0.0.1/private"},
		{target: "https://example.com/source", options: RequestOptions{Proxy: "http://localhost:8080"}},
	} {
		key := policyDocumentKey(ctx, documentKey(tc.target, "", 60, tc.options))
		encoded, _ := json.Marshal(define.RemoteBodySanitized{HTTPStatus: 200, FinalURL: tc.target, Body: "must-not-serve"})
		if err := cacher.SetWithTTL(ctx, key, string(encoded), time.Minute); err != nil {
			t.Fatal(err)
		}
		got := GetRemoteDocumentContext(ctx, tc.target, "", 60, false, tc.options)
		if got.Code == define.ERROR_CODE_NULL || got.Body != "" {
			t.Fatalf("served blocked cached result: %+v", got)
		}
	}
}
