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
	"time"
)

func TestApplicationPreservesCategoryAndRejectsInvalidDates(t *testing.T) {
	var date atomic.Value
	date.Store("1960-01-01")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<article><h2>Title</h2><a href="/post">read</a><span class="date">%s</span><span class="category">技术 &amp; Go</span></article>`, date.Load().(string))
	}))
	defer upstream.Close()
	config := testConfig(t, "contract", upstream.URL)
	file := filepath.Join(config.Server.RuleDirectory, "site", "feed.js")
	body := fmt.Sprintf(`potted.SetScope("article").SetConfig({Name:"contract",URL:%q,Mode:"ssr",Title:"h2",Link:"a",DateTime:".date",Category:".category",DisableCache:true});`, upstream.URL)
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	app, err := New(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	status, body := response(t, app, "/contract/site/json/")
	if status != 200 {
		t.Fatalf("valid feed: %d %s", status, body)
	}
	var feed struct {
		Items []struct {
			Tags      []string `json:"tags"`
			Published string   `json:"date_published"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 1 || len(feed.Items[0].Tags) != 1 || feed.Items[0].Tags[0] != "技术 & Go" {
		t.Fatalf("category lost: %s", body)
	}
	published, err := time.Parse(time.RFC3339, feed.Items[0].Published)
	expected := time.Date(1960, time.January, 1, 0, 0, 0, 0, time.Local)
	if err != nil || published.Unix() != expected.Unix() {
		t.Fatalf("negative timestamp changed: %q %v", feed.Items[0].Published, err)
	}
	date.Store("not-a-date")
	if status, body := response(t, app, "/contract/site/json/"); status != 500 || !strings.Contains(body, "generate feed failed") {
		t.Fatalf("invalid date returned partial success: %d %s", status, body)
	}
	// A semantically invalid reload must preserve the previous valid source and
	// its display metadata; feed data dates remain checked on each generation.
	if err := os.WriteFile(file, []byte(`potted.SetConfig({Name:"broken"});`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := app.rules.Reload(app.resources(t.Context())); err == nil {
		t.Fatal("missing minimum fields were published")
	}
	date.Store("1960-01-01")
	if status, body := response(t, app, "/contract/site/json/"); status != 200 || !strings.Contains(body, "contract") {
		t.Fatalf("valid source not retained: %d %s", status, body)
	}
}

func TestApplicationRejectsMissingSSRTitleAtStartup(t *testing.T) {
	config := testConfig(t, "invalid", "https://example.org")
	file := filepath.Join(config.Server.RuleDirectory, "site", "feed.js")
	if err := os.WriteFile(file, []byte(`potted.SetScope("article").SetConfig({Name:"invalid",URL:"https://example.org",Mode:"ssr"});`), 0600); err != nil {
		t.Fatal(err)
	}
	app, err := New(t.Context(), config)
	if app != nil {
		app.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "Title") {
		t.Fatalf("invalid startup: %v", err)
	}
}
