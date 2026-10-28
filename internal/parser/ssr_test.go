package parser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/goquery"
)

func TestSSRPropagatesUpstreamFailures(t *testing.T) {
	logger.Initialize()
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer upstream.Close()
			got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: upstream.URL, ListContainer: "article", Title: "h2", DisableCache: true})
			if got.Code == define.ERROR_CODE_NULL || got.Status == "" {
				t.Fatalf("upstream %d treated as success: %#v", status, got)
			}
		})
	}
	got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: "://invalid", DisableCache: true})
	if got.Code != define.ERROR_CODE_PARSE_CONTENT_FAILED {
		t.Fatalf("network error lost: %#v", got)
	}
}

func TestSSRPaginationPropagatesFailure(t *testing.T) {
	logger.Initialize()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte(`<a class="page" href="/failed">Next</a>`))
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()
	got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: upstream.URL, ListContainer: "article", Title: "h2", Pager: ".page", PagerLimit: 2, DisableCache: true})
	if got.Code != define.ERROR_CODE_API_NOT_READY {
		t.Fatalf("pagination error lost: %#v", got)
	}
}

func TestSSRPaginationBudgetAndCurrentPage(t *testing.T) {
	logger.Initialize()
	for _, currentLink := range []bool{false, true} {
		for _, limit := range []int{0, 1, 2, 3, 10} {
			t.Run(fmt.Sprintf("current=%t/limit=%d", currentLink, limit), func(t *testing.T) {
				var mu sync.Mutex
				var paths []string
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					paths = append(paths, r.URL.Path)
					mu.Unlock()
					fmt.Fprintf(w, `<article><h2>%s</h2><a href="/post">post</a></article>`, r.URL.Path)
					if r.URL.Path == "/" {
						if currentLink {
							fmt.Fprint(w, `<a class="page" href="/#top">current</a>`)
						}
						fmt.Fprint(w, `<a class="page" href="/2">2</a><a class="page" href="/2#top">2 again</a><a class="page" href="/3">3</a>`)
					}
				}))
				defer upstream.Close()
				got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: upstream.URL, Pager: ".page", PagerLimit: limit, ListContainer: "article", Title: "h2", Link: "a", DisableCache: true})
				want := limit
				if want == 0 {
					want = 1
				}
				if want > 3 {
					want = 3
				}
				mu.Lock()
				defer mu.Unlock()
				if got.Code != define.ERROR_CODE_NULL || len(got.Body) != want || len(paths) != want {
					t.Fatalf("limit %d: items=%#v paths=%v error=%s", limit, got.Body, paths, got.Status)
				}
				if got.Body[0].Title != "/" {
					t.Fatal("current page omitted")
				}
			})
		}
	}
	got := ParseDataAndConfigBySSR(define.JavaScriptConfig{URL: "https://example.org", Pager: ".missing", PagerLimit: 3, ListContainer: "article", Title: "h2"}, define.RemoteBodySanitized{}, `<article><h2>current</h2></article>`)
	if len(got.Body) != 1 || got.Body[0].Title != "current" {
		t.Fatalf("page without pagination: %#v", got)
	}
}

func TestSSRRejectsInvalidIDRegexp(t *testing.T) {
	for _, pattern := range []string{"[", `/post/\d+`} {
		got := ParseDataAndConfigBySSR(define.JavaScriptConfig{IdByRegexp: pattern, ListContainer: "article", Title: "h2", Link: "a"}, define.RemoteBodySanitized{}, `<article><h2>title</h2><a href="/post/42">post</a></article>`)
		if got.Code != define.ERROR_CODE_PARSE_CONTENT_FAILED || !strings.Contains(got.Status, "IdByRegexp") {
			t.Fatalf("%q: %#v", pattern, got)
		}
	}
	for _, link := range []string{"/post/42", "/other"} {
		got := ParseDataAndConfigBySSR(define.JavaScriptConfig{URL: "https://example.org", IdByRegexp: `/post/(\d+)`, ListContainer: "article", Title: "h2", Link: "a"}, define.RemoteBodySanitized{}, fmt.Sprintf(`<article><h2>title</h2><a href="%s">post</a></article>`, link))
		want := ""
		if link == "/post/42" {
			want = "42"
		}
		if got.Code != define.ERROR_CODE_NULL || len(got.Body) != 1 || got.Body[0].ID != want {
			t.Fatalf("%s: %#v", link, got)
		}
	}
}

func TestSSRSharedDeadlineAcrossPages(t *testing.T) {
	logger.Initialize()
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case <-time.After(60 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, `<article><h2>title</h2></article><a class="page" href="/2">2</a><a class="page" href="/3">3</a><a class="page" href="/4">4</a>`)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	began := time.Now()
	got := GetDataAndConfigBySSRContext(ctx, define.JavaScriptConfig{URL: upstream.URL, Pager: ".page", PagerLimit: 4, ListContainer: "article", Title: "h2", DisableCache: true})
	if ctx.Err() != context.DeadlineExceeded || got.Code == define.ERROR_CODE_NULL || len(got.Body) != 0 {
		t.Fatalf("deadline lost: %#v, %v", got, ctx.Err())
	}
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Fatalf("total deadline exceeded: %v", elapsed)
	}
	if requests.Load() >= 4 {
		t.Fatal("continued pagination after budget expired")
	}
}

func TestSSRDetailHookHonorsDeadline(t *testing.T) {
	logger.Initialize()
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprint(w, `<article><h2>title</h2><a href="/detail">detail</a></article>`)
			return
		}
		<-r.Context().Done()
		close(canceled)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	got := GetDataAndConfigBySSRContext(ctx, define.JavaScriptConfig{URL: upstream.URL, ListContainer: "article", Title: "h2", Link: "a", DisableCache: true, ContentHook: define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: "body"}})
	if got.Code == define.ERROR_CODE_NULL || len(got.Body) != 0 {
		t.Fatalf("partial feed accepted: %#v", got)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("upstream request not canceled")
	}
}

func TestSSRExtractsMetadataPropertiesAndDetailHooks(t *testing.T) {
	logger.Initialize()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><p class="date">2026-10-28</p><p class="category">News</p><p class="description">Summary</p><p class="content">Full article</p></body></html>`)
	}))
	defer upstream.Close()
	html := `<article data-id="entry"><h2>Title</h2><a href="/detail">read</a><b>Author</b></article>`
	hook := func(selector string) define.ConfigHook {
		return define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: selector}
	}
	config := define.JavaScriptConfig{URL: upstream.URL, ListContainer: "article", Title: "h2", Link: "a", Author: "b", DisableCache: true, IdByProp: define.PROP_ID{Prop: "data-id"}, DateTimeHook: hook(".date"), CategoryHook: hook(".category"), DescriptionHook: hook(".description"), ContentHook: hook(".content")}
	got := ParseDataAndConfigBySSR(config, define.RemoteBodySanitized{}, html)
	if got.Code != 0 || len(got.Body) != 1 {
		t.Fatalf("%#v", got)
	}
	item := got.Body[0]
	if item.ID != "entry" || item.Title != "Title" || item.Author != "Author" || item.Date != "2026-10-28" || item.Category != "News" || item.Description != "Summary" || item.Content != "Full article" {
		t.Fatalf("metadata: %#v", item)
	}
	config.DateTime = ".override-date"
	config.Category = ".override-category"
	config.Description = ".override-description"
	config.ContentHook = define.ConfigHook{}
	config.Content = ".override-content"
	got = ParseDataAndConfigBySSR(config, define.RemoteBodySanitized{}, strings.Replace(html, "</article>", `<i class="override-date">static date</i><i class="override-category">static category</i><i class="override-description">static description</i><i class="override-content">static content</i></article>`, 1))
	item = got.Body[0]
	if item.Date != "static date" || item.Category != "static category" || item.Description != "static description" || item.Content != "static content" {
		t.Fatalf("overrides: %#v", item)
	}
}
func TestSSRSelectionMethodsAndFullPage(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<article><a href="/item"> Text <b>Bold</b></a></article>`))
	if err != nil {
		t.Fatal(err)
	}
	scope := doc.Find("article")
	for _, tc := range []struct{ field, method, want string }{{"a", "text", "Text Bold"}, {"a", "href", "/item"}, {"a", "html", "Text <b>Bold</b>"}, {".missing", "href", ""}, {"固定值", "text", "固定值"}} {
		if got := strings.TrimSpace(jsBridge(tc.field, tc.method, scope)); got != tc.want {
			t.Fatalf("%+v => %q", tc, got)
		}
	}
	got := ParseFullPageByGoQuery(`<html><body><script>bad()</script><h2>ok</h2></body></html>`, func(d *goquery.Document) string { d.Find("script").Remove(); html, _ := d.Html(); return html })
	if strings.Contains(got, "bad()") || !strings.Contains(got, "ok") {
		t.Fatal(got)
	}
	if got := ParseDataAndConfigBySSR(define.JavaScriptConfig{PagerLimit: -1}, define.RemoteBodySanitized{}, "body"); got.Code == 0 {
		t.Fatal("negative budget accepted")
	}
}
