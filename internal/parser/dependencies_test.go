package parser

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/cascadia"
)

func TestUpstreamSelectorsPreserveUnicodeRules(t *testing.T) {
	config := define.JavaScriptConfig{ListContainer: `#\4f60\597d`, Title: "h2", Link: "a", URL: "https://example.org"}
	result := ParseDataAndConfigBySSR(config, define.RemoteBodySanitized{Body: `<article id="你好"><h2>中文标题</h2><a href="/post">Read</a></article>`}, "")
	if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Title != "中文标题" || result.Body[0].Link != "https://example.org/post" {
		t.Fatalf("escaped Unicode selector changed: %#v", result)
	}
}

func TestUpstreamSelectorsRejectExcessiveNesting(t *testing.T) {
	selector := strings.Repeat(":not(", 1100) + "div" + strings.Repeat(")", 1100)
	if _, err := cascadia.Compile(selector); err == nil {
		t.Fatal("unbounded selector nesting accepted")
	}
	// Six-digit escapes must parse safely even outside the Unicode scalar range.
	if _, err := cascadia.Compile(`#\ffffff`); err != nil {
		t.Fatalf("escape handling: %v", err)
	}
}
