package parser

import (
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/source"
)

func TestDeclarativeParsingRejectsDeepAndOversizedItemSets(t *testing.T) {
	spec := source.Spec{Name: "News", URL: "https://example.org", ItemSelector: "article", TitleSelector: "h2", LinkSelector: "a"}
	config, err := spec.Config()
	if err != nil {
		t.Fatal(err)
	}
	good := `<article><h2>Title</h2><a href="/post">Read</a></article>`
	for _, tc := range []struct {
		body string
		ok   bool
	}{{good, true}, {strings.Repeat(good, source.MaxItems+1), false}, {strings.Repeat("<div>", source.MaxDocumentDepth+1) + good + strings.Repeat("</div>", source.MaxDocumentDepth+1), false}} {
		doc := define.RemoteBodySanitized{Code: define.ERROR_CODE_NULL, Date: time.Now(), FinalURL: spec.URL, Body: tc.body}
		got := ParseDataAndConfigBySSRContext(t.Context(), config, doc, "")
		if (got.Code == define.ERROR_CODE_NULL) != tc.ok {
			t.Fatalf("unexpected budget result %d for %d bytes", got.Code, len(tc.body))
		}
	}
}
