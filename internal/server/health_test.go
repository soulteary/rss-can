package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/telemetry"
	"go.uber.org/zap"
)

func TestReadinessReflectsRuntimeAndCache(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runtime bool
		cache   cacher.Status
		want    int
	}{
		{"ready", true, cacher.Status{Ready: true, Backend: "redis"}, 200},
		{"degraded", true, cacher.Status{Ready: true, Backend: "memory", Degraded: true}, 200},
		{"runtime offline", false, cacher.Status{Ready: true, Backend: "memory"}, 503},
		{"cache offline", true, cacher.Status{Backend: "redis", Degraded: true}, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/ready", readiness(func(context.Context) error {
				if !tc.runtime {
					return errors.New("runtime failed")
				}
				return nil
			}, func(context.Context) cacher.Status { return tc.cache }))
			status, body := testResponse(t, app, httptest.NewRequest("GET", "/ready", nil))
			if status != tc.want || strings.Contains(body, "runtime failed") {
				t.Fatalf("%d %s", status, body)
			}
		})
	}
}
func TestReadinessHonorsParentDeadline(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 20*time.Millisecond)
		defer cancel()
		c.SetContext(ctx)
		return c.Next()
	})
	app.Get("/ready", readiness(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }, func(ctx context.Context) cacher.Status { <-ctx.Done(); return cacher.Status{} }))
	if status, _ := testResponse(t, app, httptest.NewRequest("GET", "/ready", nil)); status != 503 {
		t.Fatal(status)
	}
}
func TestProductionMonitoringRoutesAndMetrics(t *testing.T) {
	t.Setenv("RSS_ADMIN_TOKEN", "metrics-secret")
	// NewApp uses the initialized application logger.
	installTestRule(t, "https://example.org")
	app := NewApp()
	req := httptest.NewRequest("GET", "http://localhost/_/metrics/", nil)
	if status, _ := testResponse(t, app, req); status != 401 {
		t.Fatal(status)
	}
	req = httptest.NewRequest("GET", "http://localhost/_/metrics/", nil)
	req.Header.Set("Authorization", "Bearer metrics-secret")
	status, body := testResponse(t, app, req)
	if status != 200 || !strings.Contains(body, "rss_can_js_queue_seconds_bucket") || !strings.Contains(body, "rss_can_browser_active") {
		t.Fatalf("%d %s", status, body)
	}
	if status, body := testResponse(t, app, httptest.NewRequest("GET", "/_/ready/", nil)); status != 200 || !strings.Contains(body, `"ready":true`) {
		t.Fatalf("%d %s", status, body)
	}
	if status, body := testResponse(t, app, httptest.NewRequest("GET", "/_/health/", nil)); status != 200 || !strings.Contains(body, "pong") {
		t.Fatalf("%d %s", status, body)
	}
}
func TestFeedMetricsRecordFailureAndSuccess(t *testing.T) {
	app := fiber.New()
	app.Get("/feed/:id/:type/", apiRSS())
	requests := telemetry.Default.Value("feed_requests_total")
	failures := telemetry.Default.Value("feed_failures_total")
	testResponse(t, app, httptest.NewRequest("GET", "/feed/missing/invalid/", nil))
	if telemetry.Default.Value("feed_requests_total") != requests+1 || telemetry.Default.Value("feed_failures_total") != failures+1 {
		t.Fatal("feed failure not counted")
	}
}
func TestLoggerSanitizesAndPreservesErrors(t *testing.T) {
	if inputSanitized("a\r\nb") != "ab" {
		t.Fatal("log injection not sanitized")
	}
	for _, status := range []int{200, 400, 500} {
		app := fiber.New()
		app.Use(Logger(zap.NewNop().Sugar(), "/skip"))
		app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(status) })
		app.Get("/skip", func(c fiber.Ctx) error { return c.SendStatus(status) })
		app.Get("/error", func(c fiber.Ctx) error { return fiber.NewError(http.StatusTeapot, "expected") })
		for _, path := range []string{"/test", "/skip", "/error"} {
			testResponse(t, app, httptest.NewRequest("GET", path, nil))
		}
	}
}
