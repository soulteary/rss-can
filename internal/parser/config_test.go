package parser

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestSSRRejectsMissingExtractionContractWithoutPartialItems(t *testing.T) {
	for _, tc := range []struct {
		field  string
		config define.JavaScriptConfig
	}{
		{"ListContainer", define.JavaScriptConfig{Title: "h2"}},
		{"ListContainer", define.JavaScriptConfig{ListContainer: "[", Title: "h2"}},
		{"Title", define.JavaScriptConfig{ListContainer: "article"}},
	} {
		result := ParseDataAndConfigBySSRContext(t.Context(), tc.config, define.RemoteBodySanitized{}, `<article><h2>Existing</h2></article>`)
		if result.Code != define.ERROR_CODE_PARSE_CONTENT_FAILED || len(result.Body) != 0 || !strings.Contains(result.Status, tc.field) {
			t.Fatalf("%+v", result)
		}
	}
	// Parsing supplied HTML without network or relative links does not need URL.
	result := ParseDataAndConfigBySSRContext(t.Context(), define.JavaScriptConfig{ListContainer: "article", Title: "h2"}, define.RemoteBodySanitized{}, `<article><h2>Existing</h2></article>`)
	if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Title != "Existing" {
		t.Fatalf("direct extraction regressed: %+v", result)
	}
}
