package rule

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

func TestRuleEvaluationAndOptInConfigurationCache(t *testing.T) {
	logger.Initialize()
	t.Setenv("RSS_CONFIG_CACHE_TTL", "30")
	configurations.clear()
	t.Cleanup(configurations.clear)
	rule := define.RuleCache{File: "inline.js", Body: []byte(`potted.SetScope("article").SetConfig({Name:" first ", URL:" https://example.org ",Mode:"ssr",Title:"h2"});`)}
	before := telemetry.Default.Value("js_requests_total")
	a, err := GenerateConfigByRule(rule)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateConfigByRule(rule)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a.Name != "first" || a.URL != "https://example.org" || telemetry.Default.Value("js_requests_total") != before+1 {
		t.Fatal("config not trimmed/cached")
	}
	rule.Body = []byte(strings.ReplaceAll(string(rule.Body), "first", "second"))
	if v, e := GenerateConfigByRule(rule); e != nil || v.Name != "second" {
		t.Fatalf("changed config: %#v %v", v, e)
	}
	for _, body := range []string{`throw new Error("bad rule")`, `potted.GetConfig=()=>"not config";`, `potted.SetConfig({IdByRegexp:"["});`} {
		if _, err := GenerateConfigByRule(define.RuleCache{File: "invalid.js", Body: []byte(body)}); err == nil {
			t.Fatalf("invalid rule succeeded: %s", body)
		}
	}
}
func TestRuleDispatchesSSRAndCancelsBrowserModes(t *testing.T) {
	logger.Initialize()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<article><h2>title</h2><a href="/post">post</a></article>`))
	}))
	defer upstream.Close()
	result := GetWebsiteDataWithConfig(define.JavaScriptConfig{Mode: "ssr", URL: upstream.URL, ListContainer: "article", Title: "h2", DisableCache: true})
	if result.Code != 0 || len(result.Body) != 1 {
		t.Fatalf("%#v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, mode := range []string{"csr", "mix"} {
		got := GetWebsiteDataWithConfigContext(ctx, define.JavaScriptConfig{Mode: mode, Headless: "ws://127.0.0.1:1/devtools/browser/test", Proxy: "http://127.0.0.1:1", DisableCache: true})
		if got.Code == 0 {
			t.Fatalf("canceled %s succeeded", mode)
		}
	}
}
