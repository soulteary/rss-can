package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func listFixture() pageView {
	return pageView{options: DefaultOptions(), metadata: func() map[string]rule.Metadata {
		return map[string]rule.Metadata{"site": {Name: "loaded name", File: "rules/site/feed.js", Sign: "abcdef123"}}
	}}
}
func listApp(view pageView, ctx context.Context) *fiber.App {
	app := fiber.New()
	if ctx != nil {
		app.Use(func(c fiber.Ctx) error { c.SetContext(ctx); return c.Next() })
	}
	app.Get("/feed", view.listPage())
	return app
}
func TestListUsesPublishedMetadataWithoutExecutingRules(t *testing.T) {
	view := listFixture()
	view.snapshot = func() map[string]define.RuleCache { t.Fatal("metadata listing read rule bodies"); return nil }
	view.configure = func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
		t.Fatal("metadata listing executed JavaScript")
		return define.JavaScriptConfig{}, nil
	}
	status, body := testResponse(t, listApp(view, nil), httptest.NewRequest("GET", "/feed", nil))
	if status != 200 || !strings.Contains(body, "loaded name") || !strings.Contains(body, "abcdef") {
		t.Fatalf("%d %s", status, body)
	}
}
func TestListCancellationStopsBeforeRuleEvaluation(t *testing.T) {
	view := listFixture()
	view.metadata = nil
	calls := 0
	view.snapshot = func() map[string]define.RuleCache { calls++; return nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	status, body := testResponse(t, listApp(view, ctx), httptest.NewRequest("GET", "/feed", nil))
	if status != 504 || calls != 0 || strings.Contains(body, "<html") {
		t.Fatalf("status=%d calls=%d body=%s", status, calls, body)
	}
}
func TestListCancellationDuringLegacyConfigurationReturnsNoPartialPage(t *testing.T) {
	logger.Initialize()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	view := listFixture()
	view.metadata = nil
	view.snapshot = func() map[string]define.RuleCache {
		return map[string]define.RuleCache{"one": {}, "two": {}, "three": {}}
	}
	calls := 0
	view.configure = func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
		calls++
		cancel()
		return define.JavaScriptConfig{Name: "partial result"}, nil
	}
	status, body := testResponse(t, listApp(view, ctx), httptest.NewRequest("GET", "/feed", nil))
	if status != 504 || calls != 1 || strings.Contains(body, "partial result") {
		t.Fatalf("%d calls=%d %s", status, calls, body)
	}
}
func TestListAdmissionOverloadAndCanceledQueueRelease(t *testing.T) {
	view := listFixture()
	view.admission = newListAdmission()
	var release []func()
	for range ListConcurrency {
		done, err := view.admission.acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		release = append(release, done)
	}
	defer func() {
		for _, done := range release {
			done()
		}
	}()
	// An already-full wait queue must reject a new HTTP request immediately.
	for range ListQueueLimit {
		view.admission.admitted <- struct{}{}
	}
	status, _ := testResponse(t, listApp(view, nil), httptest.NewRequest("GET", "/feed", nil))
	if status != 503 {
		t.Fatalf("full queue returned %d", status)
	}
	for range ListQueueLimit {
		<-view.admission.admitted
	}
	view.options.Timeout = 10 * time.Millisecond
	status, _ = testResponse(t, listApp(view, nil), httptest.NewRequest("GET", "/feed", nil))
	if status != 504 || len(view.admission.admitted) != ListConcurrency {
		t.Fatalf("canceled waiter leaked: status=%d admitted=%d", status, len(view.admission.admitted))
	}
	for _, done := range release {
		done()
		done()
	}
	if len(view.admission.admitted) != 0 || len(view.admission.active) != 0 {
		t.Fatal("active slots leaked")
	}
	status, _ = testResponse(t, listApp(view, nil), httptest.NewRequest("GET", "/feed", nil))
	if status != 200 {
		t.Fatalf("queue did not recover: %d", status)
	}
}
func TestListEscapesMetadataAndHandlesShortSign(t *testing.T) {
	view := listFixture()
	view.metadata = func() map[string]rule.Metadata {
		return map[string]rule.Metadata{`site"unsafe`: {Name: `<script>alert("x")</script>`, File: `rules/<unsafe>.js`, Sign: "x"}}
	}
	body, err := view.renderListPage(context.Background(), GetPageByName("list.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "<script>alert") || !strings.Contains(string(body), "&lt;script&gt;") || !strings.Contains(string(body), "site%22unsafe") {
		t.Fatalf("unsafe list: %s", body)
	}
}
func TestListRejectsOversizeOutputWithoutPartialHTML(t *testing.T) {
	logger.Initialize()
	view := listFixture()
	view.metadata = func() map[string]rule.Metadata {
		return map[string]rule.Metadata{"large": {Name: strings.Repeat("x", limits.MaxFeedBytes)}}
	}
	status, body := testResponse(t, listApp(view, nil), httptest.NewRequest("GET", "/feed", nil))
	if status != 500 || strings.Contains(body, "<html") {
		t.Fatalf("oversize list: %d %s", status, body)
	}
}
