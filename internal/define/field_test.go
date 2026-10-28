package define

import "testing"

func TestFieldExpressionIntent(t *testing.T) {
	for _, tc := range []struct {
		input, value       string
		selector, explicit bool
	}{
		{"固定作者", "固定作者", false, false}, {"Yahoo", "Yahoo", false, false}, {"ARTICLE", "ARTICLE", true, false}, {"h2", "h2", true, false},
		{".entry a", ".entry a", true, false}, {"[data-title]", "[data-title]", true, false}, {"a:first-child", "a:first-child", true, false}, {"article > h2", "article > h2", true, false},
		{"css:custom-card", "custom-card", true, true}, {"css: .entry ", ".entry", true, true}, {"css:", "", true, true},
		{"literal:Fixed author", "Fixed author", false, true}, {"literal:https://example.org", "https://example.org", false, true}, {"literal:a", "a", false, true}, {"literal:", "", false, true},
		{" Author Name ", "Author Name", true, false}, {"", "", false, false}, {"\t", "", false, false},
	} {
		got := ParseFieldExpression(tc.input)
		if got.Value != tc.value || got.Selector != tc.selector || got.Explicit != tc.explicit {
			t.Fatalf("%q: %+v", tc.input, got)
		}
	}
}
