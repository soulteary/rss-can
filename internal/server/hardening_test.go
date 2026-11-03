package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestProductionConfigRequiresExplicitPost(t *testing.T) {
	logger.Initialize()
	calls := 0
	app := NewAppWithDependencies(Options{AdminToken: "secret"}, Dependencies{Reload: func(context.Context) error { calls++; return nil }})
	for _, tc := range []struct {
		method, action, origin, site string
		want                         int
	}{
		{"GET", "1", "", "", 405},
		{"POST", "", "", "", 403},
		{"POST", "1", "https://attacker.invalid", "", 403},
		{"POST", "1", "", "cross-site", 403},
		{"POST", "1", "", "", 200},
		{"POST", "1", "http://localhost", "same-origin", 200},
	} {
		req := httptest.NewRequest(tc.method, "http://localhost/config/rules/fresh/", nil)
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("X-RSS-Admin-Action", tc.action)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Sec-Fetch-Site", tc.site)
		status, _ := testResponse(t, app, req)
		if status != tc.want {
			t.Fatalf("%+v: got %d", tc, status)
		}
	}
	if calls != 2 {
		t.Fatalf("unauthorized mutation executed: %d", calls)
	}
	req := httptest.NewRequest("POST", "http://localhost/config/set-loglevel/not-a-level/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-RSS-Admin-Action", "1")
	if status, _ := testResponse(t, app, req); status != 400 {
		t.Fatalf("invalid level: %d", status)
	}
}

func TestAccessLogDoesNotLeakURLSecrets(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	app := fiber.New()
	app.Use(Logger(zap.New(core).Sugar()))
	app.Get("/s/:selector/:secret/:type", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/fail/:secret", func(c fiber.Ctx) error { return errors.New("private-upstream-secret") })
	for _, path := range []string{"/s/device/download-secret/rss?key=query-secret", "/unknown-secret", "/fail/error-secret"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Referer", "https://name:password@host.invalid/private?token=referer-secret")
		testResponse(t, app, req)
	}
	if logs.Len() != 3 {
		t.Fatalf("logs: %d", logs.Len())
	}
	for _, entry := range logs.All() {
		output := entry.Message
		for _, f := range entry.Context {
			output += " " + f.String
		}
		for _, secret := range []string{"download-secret", "query-secret", "unknown-secret", "error-secret", "referer-secret", "private-upstream-secret", "password"} {
			if strings.Contains(output, secret) {
				t.Fatalf("leaked %s: %s", secret, output)
			}
		}
	}
}
