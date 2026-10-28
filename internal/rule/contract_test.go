package rule

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestExecutableConfigContract(t *testing.T) {
	base := validRepositoryConfig("contract")
	for _, tc := range []struct {
		name, field string
		change      func(*define.JavaScriptConfig)
	}{
		{"missingURL", "URL", func(c *define.JavaScriptConfig) { c.URL = "" }},
		{"relativeURL", "URL", func(c *define.JavaScriptConfig) { c.URL = "/feed" }},
		{"nonHTTP", "URL", func(c *define.JavaScriptConfig) { c.URL = "file:///tmp/feed" }},
		{"malformedURL", "URL", func(c *define.JavaScriptConfig) { c.URL = "https://example.org:bad/" }},
		{"missingScope", "ListContainer", func(c *define.JavaScriptConfig) { c.ListContainer = " " }},
		{"badScope", "ListContainer", func(c *define.JavaScriptConfig) { c.ListContainer = "[" }},
		{"missingSSRTitle", "Title", func(c *define.JavaScriptConfig) { c.Title = " " }},
		{"missingMixTitle", "Title", func(c *define.JavaScriptConfig) { c.Mode = "mix"; c.Title = "" }},
		{"unknownMode", "Mode", func(c *define.JavaScriptConfig) { c.Mode = "remote" }},
		{"badPager", "Pager", func(c *define.JavaScriptConfig) { c.Pager = "[" }},
		{"negativePages", "PagerLimit", func(c *define.JavaScriptConfig) { c.PagerLimit = -1 }},
		{"excessPages", "PagerLimit", func(c *define.JavaScriptConfig) { c.PagerLimit = 21 }},
		{"badRegexp", "IdByRegexp", func(c *define.JavaScriptConfig) { c.IdByRegexp = "[" }},
		{"badAction", "ContentHook", func(c *define.JavaScriptConfig) { c.ContentHook.Action = "fetch" }},
		{"badObject", "ContentHook", func(c *define.JavaScriptConfig) { c.ContentHook = define.ConfigHook{Action: "readlink", Object: "["} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := base
			tc.change(&config)
			if err := ValidateConfig(config); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("%+v %v", config, err)
			}
		})
	}
	// Local sources and selector-or-literal fields are supported intentionally.
	for _, mode := range []string{"", "ssr", " SSR ", "mix", "csr"} {
		config := base
		config.Mode = mode
		config.URL = "http://127.0.0.1:8080/feed"
		config.Title = "固定标题"
		config.Author = "作者"
		config.ContentHook = define.ConfigHook{Action: "readlink"}
		if mode == "csr" {
			config.Title = ""
			config.Link = ""
		} // custom GetData remains supported.
		if err := ValidateConfig(config); err != nil {
			t.Fatalf("valid %q: %v", mode, err)
		}
	}
}

func TestRepositoryRejectsMissingContractWithoutChangingPublishedRules(t *testing.T) {
	logger.Initialize()
	root := t.TempDir()
	file := writeRepositoryRule(t, root, "site", `potted.SetScope("article").SetConfig({Name:"valid",URL:"https://example.org",Title:"h2"});`)
	repo := NewRepository(root)
	if err := repo.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := repo.SnapshotMetadata()["site"]
	for _, source := range []string{
		`potted.SetConfig({Name:"empty"});`,
		`potted.SetScope("article").SetConfig({Name:"empty",URL:"https://example.org"});`,
		`potted.SetScope("article").SetConfig({Name:"empty",URL:"https://example.org",Title:"h2",Mode:"typo"});`,
	} {
		if err := os.WriteFile(file, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if err := repo.Reload(t.Context()); err == nil {
			t.Fatalf("invalid source published: %s", source)
		}
		if got := repo.SnapshotMetadata()["site"]; got != before {
			t.Fatalf("metadata changed: %+v", got)
		}
		snapshot, _ := repo.Lookup("site")
		if !strings.Contains(string(snapshot.Body), `Name:"valid"`) {
			t.Fatal("valid source lost")
		}
	}
}

func TestRepositoryChecksDynamicAndInjectedConfigurations(t *testing.T) {
	root := t.TempDir()
	writeRepositoryRule(t, root, "site", "mock")
	valid := true
	repo := NewRepositoryWithOptions(root, func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
		config := validRepositoryConfig("dynamic")
		if !valid {
			config.Title = ""
		}
		return config, nil
	}, 0, time.Second)
	if err := repo.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := repo.Lookup("site")
	valid = false
	if _, err := repo.Configure(t.Context(), snapshot); err == nil {
		t.Fatal("dynamic invalid config returned successfully")
	}
	if err := repo.Reload(t.Context()); err == nil {
		t.Fatal("injected evaluator bypassed publish validation")
	}
}

func TestRuleEvaluationTrimsAndNormalizesBeforeValidation(t *testing.T) {
	logger.Initialize()
	snapshot := define.RuleCache{File: "inline.js", Body: []byte(`potted.SetScope(" article ").SetConfig({URL:" https://example.org ",Mode:" MIX ",Title:" h2 "});`)}
	config, err := EvaluateConfigContext(t.Context(), snapshot)
	if err != nil || config.Mode != "mix" || config.Title != "h2" || config.ListContainer != "article" {
		t.Fatalf("%+v %v", config, err)
	}
}

func TestDirectDispatchChecksContractAndCancellation(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if got := GetWebsiteDataWithConfigContext(canceled, define.JavaScriptConfig{}); got.Code != define.ERROR_CODE_NETWORK {
		t.Fatalf("cancellation lost: %+v", got)
	}
	for _, config := range []define.JavaScriptConfig{{}, {Mode: "unknown"}} {
		if got := GetWebsiteDataWithConfigContext(t.Context(), config); got.Code != define.ERROR_CODE_PARSE_CONTENT_FAILED || len(got.Body) != 0 {
			t.Fatalf("invalid direct config returned success: %+v", got)
		}
	}
	config := validRepositoryConfig("proxy")
	config.Proxy = "file:///invalid"
	if got := GetWebsiteDataWithConfigContext(t.Context(), config); got.Code != define.ERROR_CODE_INIT_NETWORK_FAILED {
		t.Fatalf("proxy error changed: %+v", got)
	}
}
