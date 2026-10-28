package rule

import (
	"github.com/soulteary/RSS-Can/internal/define"
	"strings"
	"testing"
)

func TestExplicitFieldExpressionsValidateBeforeExecution(t *testing.T) {
	valid := define.JavaScriptConfig{Mode: "ssr", URL: "https://example.org", ListContainer: "article", Title: "css:h2", Category: "literal:固定 分类", Author: "literal:Yahoo", Link: "css:a"}
	if err := ValidateConfig(valid); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		field  string
		config define.JavaScriptConfig
	}{
		{"Title", func() define.JavaScriptConfig { v := valid; v.Title = "literal:"; return v }()},
		{"Title", func() define.JavaScriptConfig { v := valid; v.Title = "css:"; return v }()},
		{"Author", func() define.JavaScriptConfig { v := valid; v.Author = "css:["; return v }()},
		{"Category", func() define.JavaScriptConfig { v := valid; v.Category = "css:"; return v }()},
		{"ContentHook.URL", func() define.JavaScriptConfig {
			v := valid
			v.ContentHook = define.ConfigHook{Action: "readlink", URL: "css:["}
			return v
		}()},
	} {
		if err := ValidateConfig(test.config); err == nil || !strings.Contains(err.Error(), test.field) {
			t.Fatalf("%s: %v", test.field, err)
		}
	}
	valid.Author = "literal:" // Optional explicit-empty fields are allowed.
	valid.ContentHook = define.ConfigHook{Action: "readlink", URL: "", Object: "body"}
	if err := ValidateConfig(valid); err != nil {
		t.Fatal(err)
	}
}
