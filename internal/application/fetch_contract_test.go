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

func writeFetchRule(t *testing.T, config Config, target, mode, extra string) {
	t.Helper()
	ready := ""
	if mode != "ssr" {
		ready = `,ReadySelector:"body"`
	}
	source := fmt.Sprintf(`potted.SetScope("article").SetConfig({Name:"fetch",URL:%q,Mode:%q,Title:"h2",Link:"a",Expire:60%s%s});`, target, mode, ready, extra)
	if err := os.WriteFile(filepath.Join(config.Server.RuleDirectory, "site", "feed.js"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestApplicationBrowserMainFailureCannotBecomeCachedSuccess(t *testing.T) {
	browser := modeBrowser(t)
	for _, mode := range []string{"csr", "mix"} {
		for _, empty := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/empty=%t", mode, empty), func(t *testing.T) {
				var code atomic.Int32
				code.Store(503)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					status := int(code.Load())
					w.WriteHeader(status)
					if status != 200 && empty {
						fmt.Fprint(w, `<html><body>Maintenance</body></html>`)
						return
					}
					title := "Recovered"
					if status != 200 {
						title = "Maintenance"
					}
					fmt.Fprintf(w, `<html><body><article><h2>%s</h2><a href="post/42">read</a></article></body></html>`, title)
				}))
				defer upstream.Close()
				config := testConfig(t, "status", upstream.URL)
				config.Server.Headless = browser
				writeFetchRule(t, config, upstream.URL+"/page", mode, "")
				app, err := New(t.Context(), config)
				if err != nil {
					t.Fatal(err)
				}
				defer app.Close()
				if status, body := response(t, app, "/status/site/json/"); status != 502 {
					t.Fatalf("failed main document became success: %d %s", status, body)
				}
				code.Store(200)
				status, body := response(t, app, "/status/site/json/")
				if status != 200 || !strings.Contains(body, "Recovered") || strings.Contains(body, "Maintenance") {
					t.Fatalf("failed document cached: %d %s", status, body)
				}
			})
		}
	}
}

func TestApplicationFinalBaseSurvivesFreshAndCachedFetch(t *testing.T) {
	browser := modeBrowser(t)
	for _, mode := range []string{"ssr", "csr", "mix"} {
		for _, withBase := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/base=%t", mode, withBase), func(t *testing.T) {
				prefix := "/news/"
				if withBase {
					prefix = "/articles/"
				}
				var pages atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/news" {
						http.Redirect(w, r, "/news/", 302)
						return
					}
					if r.URL.Path == prefix+"post/42" {
						fmt.Fprint(w, `<html><body>Detail from final base</body></html>`)
						return
					}
					if r.URL.Path != "/news/" {
						http.NotFound(w, r)
						return
					}
					pages.Add(1)
					base := ""
					if withBase {
						base = `<base href="/articles/">`
					}
					fmt.Fprintf(w, `<html><head>%s</head><body><article><h2>Title</h2><a href="post/42">read</a></article></body></html>`, base)
				}))
				defer upstream.Close()
				config := testConfig(t, "base", upstream.URL+"/news")
				config.Server.Headless = browser
				writeFetchRule(t, config, upstream.URL+"/news", mode, `,IdByRegexp:"post/([0-9]+)",ContentHook:{action:"readlink",URL:"a",object:"body"}`)
				app, err := New(t.Context(), config)
				if err != nil {
					t.Fatal(err)
				}
				defer app.Close()
				assertFeed := func() {
					status, body := response(t, app, "/base/site/json/")
					if status != 200 {
						t.Fatalf("%d %s", status, body)
					}
					var feed modeFeed
					if err := json.Unmarshal([]byte(body), &feed); err != nil || len(feed.Items) != 1 {
						t.Fatalf("%v %s", err, body)
					}
					item := feed.Items[0]
					if item.URL != upstream.URL+prefix+"post/42" || item.ID != "42" || !strings.Contains(item.Content, "Detail from final base") {
						t.Fatalf("base metadata lost: %s", body)
					}
				}
				assertFeed()
				upstream.Close() // Both source and detail must now be served from cache.
				assertFeed()
				if pages.Load() != 1 {
					t.Fatalf("source fetched %d times", pages.Load())
				}
			})
		}
	}
}

func TestApplicationRedirectDoesNotRebindCookieOrigin(t *testing.T) {
	browser := modeBrowser(t)
	for _, mode := range []string{"ssr", "csr", "mix"} {
		t.Run(mode, func(t *testing.T) {
			var leaked, originCookie atomic.Bool
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.Header.Get("Cookie"), "secret") {
					leaked.Store(true)
				}
				if r.URL.Path == "/news/post/42" {
					fmt.Fprint(w, `<html><body>Cookie-safe detail</body></html>`)
					return
				}
				fmt.Fprint(w, `<html><body><article><h2>Title</h2><a href="post/42">read</a></article></body></html>`)
			}))
			defer destination.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.Header.Get("Cookie"), "secret") {
					originCookie.Store(true)
				}
				http.Redirect(w, r, destination.URL+"/news/", 302)
			}))
			defer origin.Close()
			target := strings.Replace(origin.URL, "127.0.0.1", "localhost", 1) + "/start"
			config := testConfig(t, "cookie", target)
			config.Server.Headless = browser
			writeFetchRule(t, config, target, mode, `,Cookies:"origin=secret",DisableCache:true,ContentHook:{action:"readlink",URL:"a",object:"body"}`)
			app, err := New(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			status, body := response(t, app, "/cookie/site/json/")
			if status != 200 || !strings.Contains(body, "Cookie-safe detail") || leaked.Load() || !originCookie.Load() {
				t.Fatalf("status=%d leak=%t origin=%t %s", status, leaked.Load(), originCookie.Load(), body)
			}
		})
	}
}
