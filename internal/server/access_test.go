package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/rule"
)

type testDirectory struct {
	err     error
	onCheck func()
}

func (d *testDirectory) Active(context.Context, string) error {
	if d.onCheck != nil {
		d.onCheck()
	}
	return d.err
}

type accessGenerator struct {
	calls  int
	scope  access.Scope
	public bool
}

func (g *accessGenerator) Generate(ctx context.Context, id, format string) (string, error) {
	g.calls++
	g.scope = access.ScopeFromContext(ctx)
	g.public = network.PolicyForContext(ctx).PublicOnly()
	return id + ":" + format, nil
}

func accessApp(t *testing.T) (*fiber.App, *AccessController, *accessGenerator) {
	t.Helper()
	logger.Initialize()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := access.Open(filepath.Join(dir, "access.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, src := range []access.Source{{ID: "private-a", Owner: "alice"}, {ID: "private-b", Owner: "bob"}, {ID: "public-c", Owner: "alice", Public: true}} {
		if err := store.PutSource(src); err != nil {
			t.Fatal(err)
		}
	}
	controller := &AccessController{Config: access.Config{PublicOrigin: "https://rss.example", TrustedProxies: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/32")}}, Store: store, Directory: &testDirectory{}}
	gen := &accessGenerator{}
	opts := Options{Timeout: time.Second, FeedPath: "/feed", Access: &controller.Config, AdminToken: strings.Repeat("a", 32)}
	deps := Dependencies{Feeds: gen, Access: controller, Snapshot: func() map[string]define.RuleCache {
		return map[string]define.RuleCache{"private-a": {}, "private-b": {}, "public-c": {}}
	}, Metadata: func() map[string]rule.Metadata {
		return map[string]rule.Metadata{"private-a": {Name: "Alice private"}, "private-b": {Name: "Bob private"}, "public-c": {Name: "Public"}}
	}}
	return NewAppWithDependencies(opts, deps), controller, gen
}

func requestAs(method, path, subject, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if subject != "" {
		req.Header.Set("X-Auth-User", subject)
	}
	req.Header.Set("Content-Type", "application/json")
	if method == "POST" {
		req.Header.Set("X-RSS-Admin-Action", "1")
	}
	return req
}

func TestTeamFeedIdentityAndDirectoryIsolation(t *testing.T) {
	app, a, gen := accessApp(t)
	for _, tc := range []struct {
		subject, path string
		want          int
	}{{"", "/feed/private-a/rss", 401}, {"alice", "/feed/private-a/rss", 200}, {"bob", "/feed/private-a/rss", 404}, {"alice", "/feed/missing/rss", 404}, {"", "/feed/public-c/rss", 200}} {
		before := gen.calls
		status, _ := testResponse(t, app, requestAs("GET", tc.path, tc.subject, ""))
		if status != tc.want {
			t.Fatalf("%+v status %d", tc, status)
		}
		if tc.want != 200 && gen.calls != before {
			t.Fatal("unauthorized request generated feed")
		}
	}
	status, body := testResponse(t, app, requestAs("GET", "/feed", "alice", ""))
	if status != 200 || strings.Contains(body, "Bob private") || !strings.Contains(body, "Alice private") {
		t.Fatalf("private listing leak: %d %s", status, body)
	}
	status, _ = testResponse(t, app, requestAs("GET", "/feed/private-a/rss", "alice", ""))
	if !gen.public {
		t.Fatal("team embedder bypassed public policy")
	}
	if status != 200 || gen.scope.Key != "alice\x00private-a" || gen.scope.Version == 0 {
		t.Fatal("missing authorization cache scope")
	}
	// Role strings do not turn a reader into an administrator.
	req := requestAs("POST", "/api/access/principals", "bob", `{"subject":"alice","disabled":true}`)
	req.Header.Set("X-Auth-Role", "admin")
	status, _ = testResponse(t, app, req)
	if status != 401 {
		t.Fatal("role header elevated privileges")
	}
	// A forged marker, without a stable user ID, is insufficient.
	req = requestAs("GET", "/feed/private-a/rss", "", "")
	req.Header.Set("X-Forwarded-User", "alice")
	status, _ = testResponse(t, app, req)
	if status != 401 {
		t.Fatal("authentication marker used as subject")
	}
	a.Config.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32")}
	status, _ = testResponse(t, app, requestAs("GET", "/feed/private-a/rss", "alice", ""))
	if status != 401 {
		t.Fatal("untrusted socket supplied identity")
	}
}

func TestTeamReaderGrantLifecycleAndFormatScope(t *testing.T) {
	app, a, gen := accessApp(t)
	create := requestAs("POST", "/api/sources/private-a/grants", "alice", `{"formats":["rss"],"days":1}`)
	status, _ := testResponse(t, app, create)
	if status != 403 {
		t.Fatal("grant write without origin accepted")
	}
	create = requestAs("POST", "/api/sources/private-a/grants", "alice", `{"formats":["rss"],"days":1}`)
	create.Header.Set("Origin", "https://rss.example")
	create.Header.Del("X-RSS-Admin-Action")
	status, _ = testResponse(t, app, create)
	if status != 403 {
		t.Fatal("grant write without non-simple header accepted")
	}
	create = requestAs("POST", "/api/sources/private-a/grants", "alice", `{"formats":["rss"],"days":1}`)
	create.Header.Set("Origin", "https://rss.example")
	status, body := testResponse(t, app, create)
	if status != 201 {
		t.Fatalf("create %d %s", status, body)
	}
	var grant struct{ ID, Token, URL string }
	if err := json.Unmarshal([]byte(body), &grant); err != nil {
		t.Fatal(err)
	}
	if grant.Token == "" || strings.Contains(body, "digest") {
		t.Fatal("invalid issued token")
	}
	status, body = testResponse(t, app, requestAs("GET", grant.URL, "", ""))
	if status != 200 || body != "private-a:rss" {
		t.Fatalf("reader URL %d %s", status, body)
	}
	for _, path := range []string{"/feed/private-a/rss", "/feed/private-a/atom", "/feed/private-b/rss"} {
		req := requestAs("GET", path, "", "")
		req.Header.Set("Authorization", "Bearer "+grant.Token)
		status, _ = testResponse(t, app, req)
		want := 401
		if path == "/feed/private-a/rss" {
			want = 200
		}
		if status != want {
			t.Fatalf("%s status %d", path, status)
		}
	}
	revoke := requestAs("POST", "/api/grants/"+grant.ID+"/revoke", "bob", "{}")
	revoke.Header.Set("Origin", "https://rss.example")
	status, _ = testResponse(t, app, revoke)
	if status != 404 {
		t.Fatal("other user revoked grant")
	}
	revoke = requestAs("POST", "/api/grants/"+grant.ID+"/revoke", "alice", "{}")
	revoke.Header.Set("Origin", "https://rss.example")
	status, _ = testResponse(t, app, revoke)
	if status != 204 {
		t.Fatal("revoke failed")
	}
	before := gen.calls
	status, _ = testResponse(t, app, requestAs("GET", grant.URL, "", ""))
	if status != 401 || gen.calls != before {
		t.Fatal("revoked token generated output")
	}
	_ = a
}

func TestTeamDirectoryFailsClosedAndRevocationRace(t *testing.T) {
	app, a, gen := accessApp(t)
	g, secret, err := a.Store.CreateGrant("alice", "private-a", []string{"rss"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	path := "/s/" + g.ID + "/" + secret + "/rss"
	directory := a.Directory.(*testDirectory)
	directory.err = errors.New("timeout")
	before := gen.calls
	status, body := testResponse(t, app, requestAs("GET", path, "", ""))
	if status != 503 || gen.calls != before || strings.Contains(body, "timeout") {
		t.Fatal("directory failure did not close safely")
	}
	directory.err = access.ErrDenied
	status, _ = testResponse(t, app, requestAs("GET", path, "", ""))
	if status != 401 {
		t.Fatal("upstream disabled account accepted")
	}
	directory.err = nil
	directory.onCheck = func() {
		if err := a.Store.RevokeGrant("alice", g.ID); err != nil {
			t.Error(err)
		}
	}
	status, _ = testResponse(t, app, requestAs("GET", path, "", ""))
	if status != 401 {
		t.Fatal("concurrent revoke hidden by directory lookup")
	}
}

func TestTeamAdminChangesAndDisabledAccount(t *testing.T) {
	app, a, gen := accessApp(t)
	g, secret, _ := a.Store.CreateGrant("alice", "private-a", []string{"rss"}, time.Now().Add(time.Hour))
	req := requestAs("POST", "/api/access/principals", "", `{"subject":"alice","disabled":true}`)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	status, _ := testResponse(t, app, req)
	if status != 204 {
		t.Fatal("disable failed")
	}
	before := gen.calls
	for _, req := range []*http.Request{requestAs("GET", "/feed/private-a/rss", "alice", ""), requestAs("GET", "/s/"+g.ID+"/"+secret+"/rss", "", "")} {
		status, _ = testResponse(t, app, req)
		if status != 401 || gen.calls != before {
			t.Fatal("disabled account read private content")
		}
	}
	if err := a.Store.SetPrincipal("alice", false); err != nil {
		t.Fatal(err)
	}
	status, _ = testResponse(t, app, requestAs("GET", "/s/"+g.ID+"/"+secret+"/rss", "", ""))
	if status != 401 {
		t.Fatal("reenabling resurrected token")
	}
	// Unsupported input must not silently weaken source ACLs.
	req = requestAs("POST", "/api/access/sources", "", `{"id":"private-a","owner":"bob","visibility":"public"}`)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	status, _ = testResponse(t, app, req)
	if status != 400 {
		t.Fatal("unknown security setting accepted")
	}
}

func TestTeamMissingControllerFailsClosed(t *testing.T) {
	logger.Initialize()
	app := NewAppWithDependencies(Options{Access: &access.Config{}, FeedPath: "/feed"}, Dependencies{Feeds: &accessGenerator{}})
	status, _ := testResponse(t, app, requestAs("GET", "/feed/source/rss", "", ""))
	if status != 503 {
		t.Fatal("missing controller exposed feed")
	}
}

func TestTeamGrantQuotaReturnsActionableError(t *testing.T) {
	app, a, _ := accessApp(t)
	for i := 0; i < access.MaxGrantsPerSubject; i++ {
		if _, _, err := a.Store.CreateGrant("alice", "private-a", []string{"rss"}, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	request := requestAs("POST", "/api/sources/private-a/grants", "alice", `{"formats":["rss"]}`)
	request.Header.Set("Origin", "https://rss.example")
	status, body := testResponse(t, app, request)
	if status != http.StatusTooManyRequests || !strings.Contains(body, "GRANT_LIMIT") {
		t.Fatalf("quota response: %d %s", status, body)
	}
}
