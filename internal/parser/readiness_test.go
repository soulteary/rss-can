package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestBrowserReadySelectorContract(t *testing.T) {
	config := define.JavaScriptConfig{URL: "https://example.org", ListContainer: "article", Title: "h2"}
	if got := browserReadySelector(config); got != "article" {
		t.Fatalf("default changed: %q", got)
	}
	key := browserCacheKey(config, false)
	config.ReadySelector = "  #loaded \n"
	if got := browserReadySelector(config); got != "#loaded" {
		t.Fatalf("explicit marker: %q", got)
	}
	if err := ValidatePageConfig(config); err != nil {
		t.Fatal(err)
	}
	if browserCacheKey(config, false) == key {
		t.Fatal("readiness policy reused browser cache identity")
	}
	config.ReadySelector = "["
	if err := ValidatePageConfig(config); err == nil || !strings.Contains(err.Error(), "ReadySelector") {
		t.Fatalf("invalid marker accepted: %v", err)
	}
	if err := ValidateReadySelector(""); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserReadySelectorRejectsInvalidBeforeConnection(t *testing.T) {
	config := define.JavaScriptConfig{ReadySelector: "[", DisableCache: true}
	result := ParsePageByBrowserContext(t.Context(), config, "unreachable-browser", "", false)
	if result.Code != define.ERROR_CODE_PARSE_CONTENT_FAILED || !strings.Contains(result.Status, "ReadySelector") {
		t.Fatalf("invalid marker not rejected first: %+v", result)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result = ParsePageByBrowserContext(ctx, config, "unreachable-browser", "", false)
	if result.Code != define.ERROR_CODE_NETWORK || !strings.Contains(result.Status, "canceled") {
		t.Fatalf("caller cancellation lost precedence: %+v", result)
	}
}

func readinessBrowser(t *testing.T) string {
	t.Helper()
	address := os.Getenv("RSS_TEST_BROWSER_URL")
	if address == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("browser is required")
		}
		t.Skip("requires isolated Chrome")
	}
	logger.Initialize()
	return address
}

func readinessConfig(target, mode, marker string) define.JavaScriptConfig {
	config := define.JavaScriptConfig{Mode: mode, URL: target, ListContainer: "article", ReadySelector: marker, Title: "h2", Link: "a", DisableCache: true, Timeout: 2}
	encoded, _ := json.Marshal(config)
	config.RuleSource = define.NewRuleSource([]byte(fmt.Sprintf(`potted.SetConfig(%s).SetScope("article");`, encoded)))
	return config
}

func TestLiveBrowserReadinessAllowsExplicitEmptyAndPreservesDelayedData(t *testing.T) {
	address := readinessBrowser(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/empty" {
			fmt.Fprint(w, `<html><body><main id="loaded">No entries</main></body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body><main>Loading</main><script>setTimeout(()=>{document.body.insertAdjacentHTML("beforeend",'<article><h2>Delayed</h2><a href="/post">read</a></article>')},150)</script></body></html>`)
	}))
	defer upstream.Close()
	for _, mode := range []string{define.PARSE_MODE_CSR, define.PARSE_MODE_MIX} {
		t.Run(mode+"-empty", func(t *testing.T) {
			config := readinessConfig(upstream.URL+"/empty", mode, "#loaded")
			ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
			defer cancel()
			result := ParsePageByBrowserContext(ctx, config, address, "", mode == define.PARSE_MODE_MIX)
			if result.Code != 0 || len(result.Body) != 0 {
				t.Fatalf("explicitly ready empty page failed: %+v", result)
			}
		})
		t.Run(mode+"-delayed", func(t *testing.T) {
			config := readinessConfig(upstream.URL+"/delayed", mode, "")
			ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
			defer cancel()
			result := ParsePageByBrowserContext(ctx, config, address, "", mode == define.PARSE_MODE_MIX)
			if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Title != "Delayed" {
				t.Fatalf("default skipped delayed data: %+v", result)
			}
		})
	}
}

func TestLiveBrowserDefaultReadinessTimeoutDoesNotBecomeEmptySuccess(t *testing.T) {
	address := readinessBrowser(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><p>No item or ready marker</p></body></html>`)
	}))
	defer upstream.Close()
	config := readinessConfig(upstream.URL, define.PARSE_MODE_CSR, "")
	config.Timeout = 1
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	result := ParsePageByBrowserContext(ctx, config, address, "", false)
	if result.Code == 0 || len(result.Body) != 0 {
		t.Fatalf("unready page became empty success: %+v", result)
	}
	// Explicit marker waits also respect the caller deadline rather than returning
	// a partial/empty success when that marker never appears.
	config.ReadySelector = "#loaded"
	ctx2, cancel2 := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel2()
	result = ParsePageByBrowserContext(ctx2, config, address, "", false)
	if result.Code == 0 || len(result.Body) != 0 {
		t.Fatalf("canceled marker wait succeeded: %+v", result)
	}
}
