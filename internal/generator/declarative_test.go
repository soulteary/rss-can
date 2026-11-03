package generator

import (
	"github.com/soulteary/RSS-Can/internal/define"
	"strings"
	"testing"
)

func TestDeclarativeOutputEscapesDescriptionsAndRejectsUnsafeLinks(t *testing.T) {
	config := define.JavaScriptConfig{Name: "News", URL: "https://example.org", TextOnly: true}
	data := define.BodyParsed{Body: []define.InfoItem{{Title: "Title", Link: "https://example.org/post", Description: `<img src=x onerror=alert(1)>`}}}
	out, err := GenerateFeedsByTypeContext(t.Context(), config, data, "rss")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "<img src=") {
		t.Fatal("active HTML emitted")
	}
	data.Body[0].Link = "javascript:alert(1)"
	if _, err := GenerateFeedsByTypeContext(t.Context(), config, data, "rss"); err == nil {
		t.Fatal("unsafe link emitted")
	}
}
