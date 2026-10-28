package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestRuleReloadReportsFailure(t *testing.T) {
	logger.Initialize()
	app := fiber.New()
	app.Get("/config/:type/:value/", apiConfigWithReload(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("reload missing deadline")
		}
		return errors.New("cannot scan rules")
	}))
	status, body := testResponse(t, app, httptest.NewRequest("GET", "/config/rules/fresh/", nil))
	if status != 500 || !strings.Contains(body, "previous rules retained") {
		t.Fatalf("%d %s", status, body)
	}
}
