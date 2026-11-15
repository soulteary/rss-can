package parser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/browsertest"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/jssdk"
)

// Small upstream snapshots exercise the actual shipped rule, not a duplicate
// config in the test. Browser-only rules are verified with the browser suite.
func TestRepositoryRulesExtractUpstreamSnapshots(t *testing.T) {
	for _, tc := range []struct{ name, file, ext, id, author string }{
		{"36kr", "36kr", "xml", "4070571639067782", "36氪"},
		{"3dmgame", "3dmgame", "html", "3993074", "3DM Game"},
		{"bilibili", "bilibili", "html", "cv53915254", "辰特"},
		{"gouhuo", "gouhuo", "html", "0_20261114170119_yLo0vIF3h", "篝火营地"},
		{"baai", "hub", "html", "59705", "新智元"},
		{"geekpark", "geekpark", "xml", "373993", "极客公园"},
		{"yahoo", "yahoo", "xml", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := os.ReadFile(filepath.Join("../../rules", tc.name, tc.file+".js"))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := jssdk.RunCode(jssdk.TPL_SSR_JS, jssdk.GenerateGetConfigWithRule(rule), tc.file)
			if err != nil {
				t.Fatal(err)
			}
			var config define.JavaScriptConfig
			if err = json.Unmarshal([]byte(raw), &config); err != nil {
				t.Fatal(err)
			}
			page, err := os.ReadFile(filepath.Join("testdata/rules", tc.name+"."+tc.ext))
			if err != nil {
				t.Fatal(err)
			}
			got := ParseDataAndConfigBySSR(config, define.RemoteBodySanitized{Body: string(page)}, "")
			count := 1
			if tc.name == "gouhuo" || tc.name == "36kr" || tc.name == "yahoo" {
				count = 2
			}
			if got.Code != define.ERROR_CODE_NULL || len(got.Body) != count {
				t.Fatalf("parse: %#v", got)
			}
			for _, item := range got.Body {
				if item.ID == "" {
					t.Fatalf("missing ID: %#v", item)
				}
			}
			item := got.Body[0]
			if item.Title == "" || !strings.HasPrefix(item.Link, "http") || item.ID == "" {
				t.Fatalf("missing identity: %#v", item)
			}
			if tc.id != "" && item.ID != tc.id {
				t.Fatalf("ID: %q, want %q", item.ID, tc.id)
			}
			if tc.author != "" && item.Author != tc.author {
				t.Fatalf("author: %q, want %q", item.Author, tc.author)
			}
			if config.DateTime != "" && item.Date == "" {
				t.Fatalf("missing date: %#v", item)
			}
		})
	}
}

func TestLiveRepositoryBAAIRule(t *testing.T) {
	address := os.Getenv("RSS_TEST_BROWSER_URL")
	if address == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("browser required")
		}
		t.Skip("requires isolated Chrome")
	}
	browsertest.Lock(t, address)
	rule, err := os.ReadFile("../../rules/baai/hub.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jssdk.RunCode(jssdk.TPL_SSR_JS, jssdk.GenerateGetConfigWithRule(rule), "hub.js")
	if err != nil {
		t.Fatal(err)
	}
	var config define.JavaScriptConfig
	if err = json.Unmarshal([]byte(raw), &config); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<main><div class="story-list"><div class="story-list-container"><div class="story-item"><a href="/view/59701"><div class="story-item-main"><h6 class="story-item-title">Title</h6><span class="story-item-author-name">智源研究院</span><span class="story-item-time">2026-11-15 16:20 分享</span><p class="story-item-summary">Summary</p></div></a></div></div></div></main>`))
	}))
	defer upstream.Close()
	config.URL = upstream.URL
	config.Headless = address
	config.RuleSource = define.NewRuleSource(rule)
	config.DisableCache = true
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	got := ParsePageByBrowserContext(ctx, config, address, "", false)
	if got.Code != define.ERROR_CODE_NULL || len(got.Body) != 1 {
		t.Fatalf("parse: %#v", got)
	}
	item := got.Body[0]
	if item.ID != "59701" || item.Link != upstream.URL+"/view/59701" || item.Title != "Title" || item.Author != "智源研究院" || item.Category != "最新" || item.Date != "2026-11-15T16:20:00+08:00" {
		t.Fatalf("fields: %#v", item)
	}
}
