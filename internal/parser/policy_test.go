package parser

import (
	"context"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/network"
)

func TestPublicPolicyRejectsBrowserBeforeCacheOrConnection(t *testing.T) {
	ctx := network.WithFetchPolicy(t.Context(), network.PublicPolicy())
	for _, mix := range []bool{false, true} {
		result := ParsePageByBrowserContext(ctx, define.JavaScriptConfig{URL: "https://example.com", File: "/missing-rule.js"}, "http://127.0.0.1:1", "", mix)
		if result.Code == define.ERROR_CODE_NULL || !strings.Contains(result.Status, "disabled by public fetch policy") {
			t.Fatalf("unexpected browser result: %+v", result)
		}
	}
	err := withBrowserPage(ctx, "http://127.0.0.1:1", "", "https://example.com", "", true, func(_ctx context.Context) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "disabled by public fetch policy") {
		t.Fatalf("inspector permitted: %v", err)
	}
}
