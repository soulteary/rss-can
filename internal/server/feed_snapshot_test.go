package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/feedservice"
	"github.com/soulteary/RSS-Can/internal/logger"
)

type snapshotGenerator struct {
	calls int
	value feedservice.Representation
}

func (s *snapshotGenerator) Generate(context.Context, string, string) (string, error) {
	panic("snapshot implementation bypassed")
}
func (s *snapshotGenerator) Representation(context.Context, string, string) (feedservice.Representation, error) {
	s.calls++
	return s.value, nil
}

func TestFeedConditionalResponsesRespectAuthorization(t *testing.T) {
	logger.Initialize()
	s := &snapshotGenerator{value: feedservice.Representation{Body: "<rss/>", ETag: `"content-hash"`, ChangedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
	routes := fiber.New()
	routes.Get("/feed/:id/:type/", func(c fiber.Ctx) error {
		if c.Get("Authorization") != "Bearer reader" {
			return c.SendStatus(401)
		}
		return c.Next()
	}, apiRSSWithService(s))
	for _, tc := range []struct {
		name, auth, etag, modified string
		status                     int
	}{
		{"denied cached tag", "", `"content-hash"`, "", 401},
		{"matching", "Bearer reader", `"content-hash"`, "", 304},
		{"weak list", "Bearer reader", `"other", W/"content-hash"`, "", 304},
		{"star", "Bearer reader", "*", "", 304},
		{"modified", "Bearer reader", "", s.value.ChangedAt.Format(http.TimeFormat), 304},
		{"etag precedence", "Bearer reader", `"different"`, s.value.ChangedAt.Format(http.TimeFormat), 200},
		{"invalid date", "Bearer reader", "", "bad-date", 200},
		{"no conditions", "Bearer reader", "", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/feed/source/rss/", nil)
			req.Header.Set("Authorization", tc.auth)
			req.Header.Set("If-None-Match", tc.etag)
			req.Header.Set("If-Modified-Since", tc.modified)
			before := s.calls
			res, err := routes.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)
			if res.StatusCode != tc.status {
				t.Fatalf("status=%d body=%s", res.StatusCode, body)
			}
			if tc.status == 401 && s.calls != before {
				t.Fatal("unauthorized request reached cache")
			}
			if tc.status == 304 && len(body) != 0 {
				t.Fatal("304 has body")
			}
			if tc.status != 401 && res.Header.Get("Cache-Control") != "private, no-cache" {
				t.Fatalf("unsafe cache policy: %s", res.Header.Get("Cache-Control"))
			}
		})
	}
}

func TestFeedSnapshotPreservesStricterCachePolicyAndStaleWarning(t *testing.T) {
	logger.Initialize()
	s := &snapshotGenerator{value: feedservice.Representation{Body: "<rss/>", ETag: `"stale"`, ChangedAt: time.Now(), Stale: true}}
	routes := fiber.New()
	routes.Get("/feed/:id/:type/", func(c fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() }, apiRSSWithService(s))
	res, err := routes.Test(httptest.NewRequest(http.MethodGet, "/feed/source/rss/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("Warning") == "" {
		t.Fatal("lost private or stale response metadata")
	}
}
