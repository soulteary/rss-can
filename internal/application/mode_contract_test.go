package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func modeBrowser(t *testing.T) string {
	t.Helper()
	endpoint := os.Getenv("RSS_TEST_BROWSER_URL")
	if endpoint == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("Chrome endpoint is required")
		}
		t.Skip("requires isolated Chrome")
	}
	return endpoint
}

type modeFeed struct {
	Items []struct {
		ID      string   `json:"id"`
		URL     string   `json:"url"`
		Content string   `json:"content_html"`
		Tags    []string `json:"tags"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
	} `json:"items"`
}

func TestApplicationModesShareFieldAndEnrichmentContracts(t *testing.T) {
	browser := modeBrowser(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/post/42" {
			fmt.Fprint(w, `<html><body>Detail body</body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body><main id="ready"><article><h2>Title</h2><a href="/post/42">read</a></article></main></body></html>`)
	}))
	defer upstream.Close()
	for _, mode := range []string{"ssr", "csr", "mix"} {
		t.Run(mode, func(t *testing.T) {
			config := testConfig(t, "modes", upstream.URL)
			config.Server.Headless = browser
			ready := ""
			if mode != "ssr" {
				ready = `,ReadySelector:"#ready"`
			}
			source := fmt.Sprintf(`potted.SetScope("article").SetConfig({Name:"modes",URL:%q,Mode:%q,Title:"css:h2",Link:"css:a",Author:"literal:固定 作者",Category:"literal:固定 分类",IdByRegexp:"/post/([0-9]+)",ContentHook:{action:"readlink",URL:"css:a",object:"body"},DisableCache:true%s});`, upstream.URL, mode, ready)
			if err := os.WriteFile(filepath.Join(config.Server.RuleDirectory, "site", "feed.js"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			app, err := New(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			status, body := response(t, app, "/modes/site/json/")
			if status != 200 {
				t.Fatalf("%d %s", status, body)
			}
			var feed modeFeed
			if err := json.Unmarshal([]byte(body), &feed); err != nil {
				t.Fatal(err)
			}
			if len(feed.Items) != 1 {
				t.Fatalf("%s", body)
			}
			item := feed.Items[0]
			if item.ID != "42" || item.URL != upstream.URL+"/post/42" || !strings.Contains(item.Content, "Detail body") || len(item.Tags) != 1 || item.Tags[0] != "固定 分类" || len(item.Authors) != 1 || item.Authors[0].Name != "固定 作者" {
				t.Fatalf("mode %s diverged: %s", mode, body)
			}
		})
	}
}
func TestApplicationBrowserExplicitReadinessAllowsEmptyFeed(t *testing.T) {
	browser := modeBrowser(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><main id="ready">No entries</main></body></html>`)
	}))
	defer upstream.Close()
	for _, mode := range []string{"csr", "mix"} {
		t.Run(mode, func(t *testing.T) {
			config := testConfig(t, "empty", upstream.URL)
			config.Server.Headless = browser
			source := fmt.Sprintf(`potted.SetScope("article").SetConfig({Name:"empty",URL:%q,Mode:%q,Title:"h2",Link:"a",ReadySelector:"#ready",DisableCache:true});`, upstream.URL, mode)
			if err := os.WriteFile(filepath.Join(config.Server.RuleDirectory, "site", "feed.js"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			app, err := New(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			status, body := response(t, app, "/empty/site/json/")
			var feed modeFeed
			if status != 200 {
				t.Fatalf("%d %s", status, body)
			}
			if err := json.Unmarshal([]byte(body), &feed); err != nil || len(feed.Items) != 0 {
				t.Fatalf("%v %s", err, body)
			}
		})
	}
}
func TestApplicationCustomCSRPreservesExplicitOutput(t *testing.T) {
	browser := modeBrowser(t)
	var details atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/post/42" {
			details.Add(1)
		}
		fmt.Fprint(w, `<html><body><article><a href="/post/42">read</a></article></body></html>`)
	}))
	defer upstream.Close()
	config := testConfig(t, "custom", upstream.URL)
	config.Server.Headless = browser
	source := fmt.Sprintf(`potted.SetScope("article").SetConfig({Name:"custom",URL:%q,Mode:"csr",IdByRegexp:"/post/([0-9]+)",ContentHook:{action:"readlink",URL:"css:a",object:"body"},DisableCache:true});potted.GetData=function(){this.value=JSON.stringify([{id:"custom-id",title:"Custom",link:"/post/42",content:"Own content",author:"Own author",category:"Own category"}]);return this;};`, upstream.URL)
	if err := os.WriteFile(filepath.Join(config.Server.RuleDirectory, "site", "feed.js"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	app, err := New(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	status, body := response(t, app, "/custom/site/json/")
	if status != 200 {
		t.Fatalf("%d %s", status, body)
	}
	var feed modeFeed
	if err := json.Unmarshal([]byte(body), &feed); err != nil || len(feed.Items) != 1 {
		t.Fatalf("%v %s", err, body)
	}
	if item := feed.Items[0]; item.ID != "custom-id" || item.URL != upstream.URL+"/post/42" || item.Content != "Own content" || details.Load() != 0 {
		t.Fatalf("custom output overwritten: %s detailrequests=%d", body, details.Load())
	}
}
