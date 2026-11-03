package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/feedservice"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/source"
)

func sourceBody(name string) string {
	return fmt.Sprintf(`{"spec":{"name":%q,"url":"https://example.org/news","item_selector":"article","title_selector":"h2","link_selector":"a"}}`, name)
}
func TestSourceWizardCreateEditRollbackAndProtectOwnership(t *testing.T) {
	app, a, _ := accessApp(t)
	a.Preview = func(context.Context, source.Spec) ([]define.InfoItem, error) {
		return []define.InfoItem{{Title: "News", Link: "https://example.org/post"}}, nil
	}
	post := func(path, who, body string) (int, string) {
		r := requestAs("POST", path, who, body)
		r.Header.Set("Origin", "https://rss.example")
		return testResponse(t, app, r)
	}
	status, body := post("/api/sources", "alice", sourceBody("Original"))
	if status != 201 {
		t.Fatalf("create %d %s", status, body)
	}
	var result struct {
		Source access.Source `json:"source"`
	}
	_ = json.Unmarshal([]byte(body), &result)
	src := result.Source
	if src.ID == "" || src.Public || src.Spec == nil {
		t.Fatal("unsafe default source")
	}
	status, _ = post("/api/sources/"+src.ID, "bob", strings.TrimSuffix(sourceBody("Stolen"), "}")+`,"revision":1}`)
	if status != 404 {
		t.Fatal("cross-owner edit accepted")
	}
	status, _ = post("/api/sources/"+src.ID, "alice", strings.TrimSuffix(sourceBody("Changed"), "}")+`,"revision":1}`)
	if status != 200 {
		t.Fatal("update failed")
	}
	status, _ = post("/api/sources/"+src.ID, "alice", strings.TrimSuffix(sourceBody("Lost"), "}")+`,"revision":1}`)
	if status != 409 {
		t.Fatal("lost update accepted")
	}
	status, _ = post("/api/sources/"+src.ID+"/rollback", "alice", `{"revision":2}`)
	if status != 200 {
		t.Fatal("rollback failed")
	}
	current, _ := a.Store.Managed(src.ID)
	if current.Spec.Name != "Original" {
		t.Fatal("rollback did not restore")
	}
	status, body = testResponse(t, app, requestAs("GET", "/sources", "alice", ""))
	if status != 200 || !strings.Contains(body, "source-form") {
		t.Fatal("wizard unavailable")
	}
}

func TestSourcePreviewRejectsUnsafeInputsBeforeFetching(t *testing.T) {
	app, a, _ := accessApp(t)
	calls := 0
	a.Preview = func(context.Context, source.Spec) ([]define.InfoItem, error) {
		calls++
		return []define.InfoItem{{Title: "News", Link: "https://example.org/post"}}, nil
	}
	for _, body := range []string{strings.Replace(sourceBody("Local"), "https://example.org/news", "http://127.0.0.1/", 1), strings.Replace(sourceBody("Code"), `"link_selector":"a"`, `"link_selector":"a","javascript":"evil()"`, 1)} {
		r := requestAs("POST", "/api/sources/preview", "alice", body)
		r.Header.Set("Origin", "https://rss.example")
		status, _ := testResponse(t, app, r)
		if status != 400 {
			t.Fatalf("unsafe preview %d", status)
		}
	}
	if calls != 0 {
		t.Fatal("unsafe preview allocated fetch")
	}
	a.Preview = func(context.Context, source.Spec) ([]define.InfoItem, error) { return nil, nil }
	r := requestAs("POST", "/api/sources/preview", "alice", sourceBody("Empty"))
	r.Header.Set("Origin", "https://rss.example")
	status, body := testResponse(t, app, r)
	if status != 422 || !strings.Contains(body, "RULE_NO_MATCH") {
		t.Fatal("empty preview not explained")
	}
}

func TestSubjectWriteLimiterIsBoundedAndNotPerToken(t *testing.T) {
	l := subjectWriteLimiter{}
	now := time.Now()
	for range writeBurst {
		if !l.allow("alice", now) {
			t.Fatal("early rate limit")
		}
	}
	if l.allow("alice", now) {
		t.Fatal("burst exceeded")
	}
	if !l.allow("bob", now) {
		t.Fatal("one user blocked another")
	}
	if !l.allow("alice", now.Add(5*time.Second)) {
		t.Fatal("bucket failed to refill")
	}
	for i := 0; i < maxWriteSubjects; i++ {
		l.allow(fmt.Sprint(i), now)
	}
	if len(l.entries) > maxWriteSubjects || l.allow("new", now) {
		t.Fatal("unbounded subject map")
	}
	if !l.allow("new", now.Add(16*time.Minute)) {
		t.Fatal("expired entries not reclaimed")
	}
}

type refreshRecorder struct {
	scope   access.Scope
	calls   int
	failure bool
}

func (r *refreshRecorder) Generate(context.Context, string, string) (string, error) { return "", nil }
func (r *refreshRecorder) Refresh(ctx context.Context, id, format string) (feedservice.Representation, error) {
	r.calls++
	r.scope = access.ScopeFromContext(ctx)
	if r.failure {
		return feedservice.Representation{}, fmt.Errorf("secret upstream failure")
	}
	return feedservice.Representation{FetchedAt: time.Now(), ChangedAt: time.Now(), Stale: false}, nil
}

func TestSourceRefreshRequiresOwnerAndCarriesScope(t *testing.T) {
	_, a, _ := accessApp(t)
	recorder := &refreshRecorder{}
	app := fiber.New()
	app.Post("/api/sources/:id/refresh", a.requireSubject(), a.refreshManaged(recorder))
	call := func(who string) (int, string) {
		req := requestAs("POST", "/api/sources/private-a/refresh", who, "{}")
		req.Header.Set("Origin", "https://rss.example")
		return testResponse(t, app, req)
	}
	if status, _ := call("bob"); status != 404 || recorder.calls != 0 {
		t.Fatal("non-owner refreshed source")
	}
	status, body := call("alice")
	if status != 200 || !strings.Contains(body, "fetched_at") || recorder.scope.Key != "alice\x00private-a" || recorder.scope.Version == 0 {
		t.Fatalf("bad refresh: %d %s %+v", status, body, recorder.scope)
	}
	recorder.failure = true
	status, body = call("alice")
	if status != 503 || strings.Contains(body, "secret") {
		t.Fatal("refresh error leaked details")
	}
}
