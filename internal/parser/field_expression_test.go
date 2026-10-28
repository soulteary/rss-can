package parser

import (
	"strings"
	"testing"

	"github.com/soulteary/goquery"
)

func TestSSRFieldExpressionExtraction(t *testing.T) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(`<article href="/self"><h2>Title</h2><a href="/post/42">link</a><span class="label">Label</span></article>`))
	if err != nil {
		t.Fatal(err)
	}
	selection := document.Find("article")
	for _, tc := range []struct{ field, method, want string }{
		{"固定作者", "text", "固定作者"}, {"Yahoo", "text", "Yahoo"}, {"literal:a", "text", "a"}, {"literal:Fixed author", "text", "Fixed author"},
		{"h2", "text", "Title"}, {"css:.label", "text", "Label"}, {"css:a", "href", "/post/42"}, {"", "href", "/self"}, {"css:", "text", ""}, {"css:.missing", "href", ""}, {"css:.label", "html", "Label"}, {"h2", "unknown", ""},
	} {
		if got := jsBridge(tc.field, tc.method, selection); got != tc.want {
			t.Fatalf("%q/%s: %q want %q", tc.field, tc.method, got, tc.want)
		}
	}
}
