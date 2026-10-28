package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/feedservice"
	"github.com/soulteary/RSS-Can/internal/logger"
)

type feedStub func(context.Context, string, string) (string, error)

func (f feedStub) Generate(ctx context.Context, id, format string) (string, error) {
	return f(ctx, id, format)
}
func TestFeedServiceHTTPMapping(t *testing.T) {
	logger.Initialize()
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"missing", &feedservice.Error{Stage: feedservice.MissingRule, Cause: errors.New("missing")}, 404, "rule not found"},
		{"config", &feedservice.Error{Stage: feedservice.Configuration, Cause: errors.New("bad config")}, 500, "parse config failed"},
		{"upstream", &feedservice.Error{Stage: feedservice.Fetch, Code: define.ERROR_CODE_NETWORK, Cause: errors.New("offline")}, 502, "get website data failed"},
		{"overload", &feedservice.Error{Stage: feedservice.Fetch, Code: define.ERROR_CODE_OVERLOADED, Cause: errors.New("full")}, 503, "upstream fetch queue is full"},
		{"timeout", &feedservice.Error{Stage: feedservice.Timeout, Cause: context.DeadlineExceeded}, 504, "feed generation timed out"},
		{"serialize", &feedservice.Error{Stage: feedservice.Serialization, Cause: errors.New("bad output")}, 500, "generate feed failed"},
		{"unknown", errors.New("unexpected"), 500, "generate feed failed"},
		{"success", nil, 200, "generated feed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/feed/:id/:type/", apiRSSWithService(feedStub(func(ctx context.Context, id, format string) (string, error) {
				if id != "site" || format != "rss" {
					t.Errorf("%s %s", id, format)
				}
				return "generated feed", tc.err
			})))
			status, body := testResponse(t, app, httptest.NewRequest("GET", "/feed/site/RSS/", nil))
			if status != tc.status || !strings.Contains(body, tc.message) {
				t.Fatalf("%d %s", status, body)
			}
		})
	}
}
