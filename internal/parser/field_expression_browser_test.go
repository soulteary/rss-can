package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestLiveNativeCSRFieldExpressionsAndHookMetadata(t *testing.T) {
	address := os.Getenv("RSS_TEST_BROWSER_URL")
	if address == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("browser required")
		}
		t.Skip("requires isolated Chrome")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><article href="/self"><h2>Title</h2><a href="/post/42">read</a><span class="label">Summary</span><custom-card>Custom</custom-card></article></body></html>`)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err := withBrowserPage(ctx, address, "", upstream.URL, "", false, func(page context.Context) error {
		if err := chromedp.Do(page, chromedp.Navigate(upstream.URL), chromedp.WaitReady(chromedp.CSS("article"))); err != nil {
			return err
		}
		source := []byte(`potted.SetScope("article").SetConfig({Title:"css:h2",Link:"css:a",Author:"literal:Yahoo",Category:"固定分类",DateTime:"literal:2026-10-29",Description:"css:.label",DateTimeHook:{action:"readlink",URL:"[unused"},CategoryHook:{action:"readlink",URL:"[bad",object:"body"},DescriptionHook:{action:"readlink",URL:"[unused"},ContentHook:{action:"readlink",URL:"literal:/detail"}});`)
		raw, err := chromedp.Run(page, chromedp.Evaluate[string]("("+jssdk.GenerateCSRInjectParser(source)+")()"))
		if err != nil {
			return err
		}
		var items []struct {
			Title, Link, Author, Category, Date, Description string
			Native                                           bool              `json:"__rsscan_native"`
			Hooks                                            map[string]string `json:"__rsscan_hooks"`
		}
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			return err
		}
		if len(items) != 1 {
			t.Fatalf("unexpected native data: %s", raw)
		}
		item := items[0]
		if item.Title != "Title" || item.Link != "/post/42" || item.Author != "Yahoo" || item.Category != "固定分类" || item.Date != "2026-10-29" || item.Description != "Summary" || !item.Native {
			t.Fatalf("field semantics: %s", raw)
		}
		if item.Hooks["Content"] != "/detail" || len(item.Hooks) != 1 {
			t.Fatalf("inline fields unexpectedly evaluated their overridden hooks: %s", raw)
		}
		hookRaw, err := chromedp.Run(page, chromedp.Evaluate[string]("("+jssdk.GenerateCSRInjectParser([]byte(`potted.SetScope("article").SetConfig({Title:"h2",Link:"a",DateTimeHook:{action:"readlink",URL:""},CategoryHook:{action:"readlink",URL:"css:a"},DescriptionHook:{action:"readlink",URL:"literal:/description"}});`))+")()"))
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(hookRaw), &items); err != nil {
			return err
		}
		for field, want := range map[string]string{"DateTime": "/self", "Category": "/post/42", "Description": "/description"} {
			if len(items) != 1 || items[0].Hooks[field] != want {
				t.Fatalf("hook %s did not capture raw href: %s", field, hookRaw)
			}
		}

		empty, err := chromedp.Run(page, chromedp.Evaluate[string]("("+jssdk.GenerateCSRInjectParser([]byte(`potted.SetScope(".missing").SetConfig({});`))+")()"))
		if err != nil {
			return err
		}
		if empty != "[]" {
			t.Fatalf("empty native scope: %q", empty)
		}
		_, err = chromedp.Run(page, chromedp.Evaluate[string]("("+jssdk.GenerateCSRInjectParser([]byte(`potted.SetScope("article").SetConfig({Title:"h2"});`))+")()"))
		if err == nil || !strings.Contains(err.Error(), "requires Title and Link") {
			t.Fatalf("native missing expression: %v", err)
		}
		for _, invalid := range []string{`potted.SetScope("article").SetConfig({Title:"literal:",Link:"a"});`, `potted.SetScope("article").SetConfig({Title:"h2",Link:"literal:"});`, `potted.SetScope("article").SetConfig({Title:"h2",Link:"css: "});`} {
			_, err := chromedp.Run(page, chromedp.Evaluate[string]("("+jssdk.GenerateCSRInjectParser([]byte(invalid))+")()"))
			if err == nil || !strings.Contains(err.Error(), "requires Title and Link") {
				t.Fatalf("native empty expression accepted: %s: %v", invalid, err)
			}
		}
		custom, err := chromedp.Run(page, chromedp.Evaluate[string]("("+jssdk.GenerateCSRInjectParser([]byte(`potted.SetScope("article").SetConfig({});potted.GetData=function(){this.value=JSON.stringify([{title:this.GetText("custom-card",""),id:"existing",link:this.GetLink("a",""),content:"provided"}]);return this;};`))+")()"))
		if err != nil {
			return err
		}
		if !strings.Contains(custom, "existing") || !strings.Contains(custom, "Custom") || !strings.Contains(custom, "/post/42") || strings.Contains(custom, "__rsscan_native") {
			t.Fatalf("custom override changed: %s", custom)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
