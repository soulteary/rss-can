package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestHomeAndInspectorPreparation(t *testing.T) {
	logger.Initialize()
	previous := define.DEBUG_MODE
	defer func() { define.DEBUG_MODE = previous }()
	for _, debug := range []bool{false, true} {
		define.DEBUG_MODE = debug
		app := fiber.New()
		app.Get("/", welcomePage())
		app.Post("/inspector", inspectorPrepare())
		if status, body := testResponse(t, app, httptest.NewRequest("GET", "/", nil)); status != 200 || !strings.Contains(body, "RSS Can") {
			t.Fatalf("%d %s", status, body)
		}
		for _, tc := range []struct {
			form    string
			encoded bool
		}{{"url=http%3A%2F%2F127.0.0.1", false}, {"url=https%3A%2F%2Fexample.org&engine=bad", false}, {"url=https%3A%2F%2Fexample.org&engine=ssr", true}} {
			req := httptest.NewRequest("POST", "/inspector", strings.NewReader(tc.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 302 || strings.Contains(response.Header.Get("Location"), "/inspector/") != tc.encoded {
				t.Fatalf("%s: %d %s", tc.form, response.StatusCode, response.Header.Get("Location"))
			}
		}
	}
	if len(GetPageByName("missing.html")) != 0 {
		t.Fatal("missing template exists")
	}
}
func TestConfigurationRoutes(t *testing.T) {
	logger.Initialize()
	previous := logger.GetLevel()
	defer logger.SetLevel(previous)
	app := fiber.New()
	app.Get("/config/:type/:value/", apiConfig())
	for _, tc := range []struct {
		path   string
		status int
	}{{"/config/set-loglevel/debug/", 200}, {"/config/rules/fresh/", 200}, {"/config/rules/bad/", 404}, {"/config/unknown/value/", 404}} {
		status, _ := testResponse(t, app, httptest.NewRequest("GET", tc.path, nil))
		if status != tc.status {
			t.Fatalf("%s: %d", tc.path, status)
		}
	}
}
