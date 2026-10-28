package rule

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestReadySelectorIsBrowserOnlyAndValidated(t *testing.T) {
	config := define.JavaScriptConfig{URL: "https://example.org", Mode: "ssr", ListContainer: "article", Title: "h2", ReadySelector: "body"}
	if err := ValidateConfig(config); err == nil || !strings.Contains(err.Error(), "ReadySelector") {
		t.Fatalf("SSR silently accepted browser setting: %v", err)
	}
	for _, mode := range []string{"csr", "mix"} {
		config.Mode = mode
		if err := ValidateConfig(config); err != nil {
			t.Fatalf("%s marker rejected: %v", mode, err)
		}
		config.ReadySelector = "["
		if err := ValidateConfig(config); err == nil || !strings.Contains(err.Error(), "ReadySelector") {
			t.Fatalf("%s bad marker accepted: %v", mode, err)
		}
		config.ReadySelector = "body"
	}
}
