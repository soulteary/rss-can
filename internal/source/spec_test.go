package source

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
)

func goodSpec() Spec {
	return Spec{Name: "News", URL: "https://example.org/news", ItemSelector: "article", TitleSelector: "h2", LinkSelector: "a"}.Normalized()
}
func TestDeclarativeSpecHasNoExecutableFields(t *testing.T) {
	s := goodSpec()
	c, err := s.Config()
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != "ssr" || c.Title != "css:h2" || !c.TextOnly || c.RuleSource.Present() || c.Proxy != "" || c.Cookies != "" || c.Headless != "" {
		t.Fatalf("unexpected executable config: %+v", c)
	}
	body, err := s.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(body)
	if err != nil || parsed != s {
		t.Fatalf("roundtrip %+v %v", parsed, err)
	}
	var fields map[string]any
	_ = json.Unmarshal(body, &fields)
	fields["javascript"] = "while(true){}"
	bad, _ := json.Marshal(fields)
	if _, err := Parse(bad); err == nil {
		t.Fatal("executable field accepted")
	}
	if _, err := Parse(append(body, []byte(" {}")...)); err == nil {
		t.Fatal("trailing object accepted")
	}
}
func TestDeclarativeSpecValidation(t *testing.T) {
	for _, change := range []func(*Spec){func(s *Spec) { s.Name = "" }, func(s *Spec) { s.URL = "file:///etc/passwd" }, func(s *Spec) { s.URL = "https://user:secret@example.org" }, func(s *Spec) { s.URL = "https://example.org/#x" }, func(s *Spec) { s.ItemSelector = "[" }, func(s *Spec) { s.TitleSelector = "" }, func(s *Spec) { s.MaxPages = 21 }, func(s *Spec) { s.RefreshSeconds = 59 }} {
		s := goodSpec()
		change(&s)
		if s.Validate() == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	if ValidateItems([]define.InfoItem{{Title: "item", Link: "javascript:alert(1)"}}) == nil {
		t.Fatal("unsafe link accepted")
	}
	if ValidateItems([]define.InfoItem{{Title: "item", Link: "https://example.org/post"}}) != nil {
		t.Fatal("valid item denied")
	}
	if ValidateItems(nil) == nil {
		t.Fatal("empty result accepted")
	}
}

func TestSelectorsCannotTriggerCombinatorBacktracking(t *testing.T) {
	for _, selector := range []string{strings.Repeat("div:has(", 10) + "span.never" + strings.Repeat(")", 10), "span.never " + strings.Repeat("div ", 10), `div\3a has(span)`, "div > span", "a[href]", "a + a", "a,div", "a:not(div)"} {
		s := goodSpec()
		s.ItemSelector = selector
		if s.Validate() == nil {
			t.Fatalf("complex selector allowed: %q", selector)
		}
	}
	for _, selector := range []string{"article.card", ".entry", "#news", "h2.title", "a.link.primary"} {
		s := goodSpec()
		s.ItemSelector = selector
		if err := s.Validate(); err != nil {
			t.Fatalf("simple selector rejected: %q: %v", selector, err)
		}
	}
}

func TestDocumentComplexityBudget(t *testing.T) {
	if err := ValidateDocument(`<article><h2>Title</h2><a href="/item">Read</a></article>`); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{strings.Repeat("x", MaxDocumentBytes+1), strings.Repeat("<div>", MaxDocumentDepth+1) + strings.Repeat("</div>", MaxDocumentDepth+1), strings.Repeat("<br>", MaxDocumentNodes+1)} {
		if ValidateDocument(body) == nil {
			t.Fatal("complex document accepted")
		}
	}
}
