package generator

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
)

func TestGeneratorRejectsItemAndOutputLimitsWithoutPartialFeed(t *testing.T) {
	if out, err := GenerateFeedsByTypeContext(t.Context(), define.JavaScriptConfig{}, define.BodyParsed{Body: make([]define.InfoItem, limits.MaxItems+1)}, "rss"); err == nil || out != "" {
		t.Fatal("too many items accepted")
	}
	for _, format := range []string{"rss", "atom", "json"} {
		out, err := GenerateFeedsByTypeContext(t.Context(), define.JavaScriptConfig{}, define.BodyParsed{Body: []define.InfoItem{{Content: strings.Repeat("x", limits.MaxFeedBytes+1)}}}, format)
		if err == nil || out != "" {
			t.Fatalf("oversized %s accepted", format)
		}
	}
	var b bytes.Buffer
	b.WriteString(strings.Repeat("x", limits.MaxFeedBytes-1))
	writer := contextWriter{ctx: context.Background(), buffer: &b}
	if _, err := writer.Write([]byte("xx")); err == nil || b.Len() != limits.MaxFeedBytes-1 {
		t.Fatal("overflow write changed buffer")
	}
}
