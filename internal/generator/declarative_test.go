package generator

import (
	"context"
	"encoding/json"
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
	jsonBody, err := GenerateFeedsByTypeContext(t.Context(), config, data, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Items []struct {
			Summary string `json:"summary"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(jsonBody), &decoded); err != nil || len(decoded.Items) != 1 || decoded.Items[0].Summary != data.Body[0].Description {
		t.Fatalf("plain summary was double escaped: %s %v", jsonBody, err)
	}
	data.Body[0].Link = "javascript:alert(1)"
	if _, err := GenerateFeedsByTypeContext(t.Context(), config, data, "rss"); err == nil {
		t.Fatal("unsafe link emitted")
	}
}

func TestDeclarativeTextBudgetAndCancellation(t *testing.T) {
	if _, err := escapeFeedText(t.Context(), strings.Repeat("&", 100), 100); err == nil {
		t.Fatal("escaping exceeded budget")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := escapeFeedText(ctx, "text", 100); err == nil {
		t.Fatal("cancellation ignored")
	}
}
