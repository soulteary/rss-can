package generator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

func TestUpstreamFeedsPreserveEntries(t *testing.T) {
	logger.Initialize()
	config := define.JavaScriptConfig{Name: "中文订阅", URL: "https://example.org"}
	data := define.BodyParsed{Body: []define.InfoItem{{ID: "entry-1", Title: "标题 & text", Link: "https://example.org/post", Author: "Author", Description: "Summary", Content: "Article content"}}}
	for _, format := range []string{define.FEED_TYPE_RSS, define.FEED_TYPE_ATOM} {
		t.Run(format, func(t *testing.T) {
			output, err := GenerateFeedsByType(config, data, format)
			if err != nil {
				t.Fatal(err)
			}
			var document struct{ XMLName xml.Name }
			if err := xml.Unmarshal([]byte(output), &document); err != nil {
				t.Fatalf("invalid XML: %v\n%s", err, output)
			}
			for _, fragment := range []string{"中文订阅", "entry-1", "https://example.org/post", "Article content"} {
				if !strings.Contains(output, fragment) {
					t.Errorf("missing %s in %s", fragment, output)
				}
			}
		})
	}
	output, err := GenerateFeedsByType(config, data, define.FEED_TYPE_JSON)
	if err != nil {
		t.Fatal(err)
	}
	var feed struct {
		Version string `json:"version"`
		Title   string `json:"title"`
		Items   []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Content string `json:"content_html"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(output), &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Version != "https://jsonfeed.org/version/1.1" || feed.Title != config.Name || len(feed.Items) != 1 {
		t.Fatalf("JSON feed changed: %s", output)
	}
	item := feed.Items[0]
	if item.ID != "entry-1" || item.Title != "标题 & text" || item.Content != "Article content" || len(item.Authors) != 1 || item.Authors[0].Name != "Author" {
		t.Fatalf("JSON entry changed: %s", output)
	}
}

func TestUpstreamFeedsHandleMissingLinks(t *testing.T) {
	logger.Initialize()
	for _, format := range []string{define.FEED_TYPE_RSS, define.FEED_TYPE_ATOM, define.FEED_TYPE_JSON} {
		if output, err := GenerateFeedsByType(define.JavaScriptConfig{Name: "Empty"}, define.BodyParsed{}, format); err != nil || output == "" {
			t.Errorf("empty %s feed could not be serialized", format)
		}
	}
}

func TestGeneratorReturnsErrors(t *testing.T) {
	if output, err := GenerateFeedsByType(define.JavaScriptConfig{}, define.BodyParsed{}, "invalid"); err == nil || output != "" {
		t.Fatalf("invalid format: %q, %v", output, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, format := range []string{define.FEED_TYPE_RSS, define.FEED_TYPE_ATOM, define.FEED_TYPE_JSON} {
		if output, err := GenerateFeedsByTypeContext(ctx, define.JavaScriptConfig{}, define.BodyParsed{}, format); !errors.Is(err, context.Canceled) || output != "" {
			t.Fatalf("canceled %s feed: %q, %v", format, output, err)
		}
	}
}

func TestRepeatedDatesUseOneConversionPerFeed(t *testing.T) {
	data := define.BodyParsed{Body: []define.InfoItem{{Title: "a", Date: "2026-10-28 12:00:00"}, {Title: "b", Date: "2026-10-28 12:00:00"}}}
	before := telemetry.Default.Value("js_requests_total")
	if _, err := GenerateFeedsByType(define.JavaScriptConfig{Name: "dates"}, data, define.FEED_TYPE_RSS); err != nil {
		t.Fatal(err)
	}
	if telemetry.Default.Value("js_requests_total") != before+1 {
		t.Fatal("repeated dates evaluated more than once")
	}
	if _, err := GenerateFeedsByType(define.JavaScriptConfig{Name: "dates"}, data, define.FEED_TYPE_RSS); err != nil {
		t.Fatal(err)
	}
	if telemetry.Default.Value("js_requests_total") != before+2 {
		t.Fatal("date cached across feeds; relative timestamps could become stale")
	}
}
