package server

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/parser"
)

func TestInspectorOverloadHTTPStatus(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"overload", fmt.Errorf("browser operation: %w", parser.ErrBrowserOverloaded), 503, "browser queue is full"},
		{"upstream", errors.New("unavailable"), 502, "inspector fetch failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/inspector/:url/", inspectorProxyWithFetcher(func(ctx context.Context, url string) (string, error) {
				if url != "https://example.org/" {
					t.Errorf("unexpected target %s", url)
				}
				return "", test.err
			}))
			status, body := testResponse(t, app, httptest.NewRequest("GET", "/inspector/"+fn.Base64Encode("https://example.org/")+"/", nil))
			if status != test.status || !strings.Contains(body, test.message) {
				t.Fatalf("status=%d body=%s", status, body)
			}
		})
	}
}
