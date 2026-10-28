package parser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestDetailFailureRejectsPartialFeedForEveryHook(t *testing.T) {
	logger.Initialize()
	for _, field := range []string{"date", "category", "description", "content"} {
		for _, failure := range []string{"upstream", "missing-selector", "missing-link"} {
			t.Run(field+"/"+failure, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/" {
						fmt.Fprint(w, `<article><h2>Title</h2><a href="/detail">read</a></article>`)
						return
					}
					if failure == "upstream" {
						w.WriteHeader(503)
						return
					}
					fmt.Fprint(w, `<body>no matching article</body>`)
				}))
				defer server.Close()
				hook := define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: ".required"}
				if failure == "missing-link" {
					hook.URL = ".absent"
				}
				config := define.JavaScriptConfig{URL: server.URL, ListContainer: "article", Title: "h2", DisableCache: true}
				switch field {
				case "date":
					config.DateTimeHook = hook
				case "category":
					config.CategoryHook = hook
				case "description":
					config.DescriptionHook = hook
				case "content":
					config.ContentHook = hook
				}
				got := GetDataAndConfigBySSR(config)
				if got.Code == 0 || len(got.Body) != 0 || !strings.Contains(got.Status, "detail") {
					t.Fatalf("partial success: %#v", got)
				}
			})
		}
	}
}

func TestSSRDetailsRunConcurrentlyAndKeepOrder(t *testing.T) {
	logger.Initialize()
	var active, peak atomic.Int32
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			for i := 0; i < 8; i++ {
				fmt.Fprintf(w, `<article><h2>%d</h2><a href="/detail/%d">read</a></article>`, i, i)
			}
			return
		}
		count := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); count > old && !peak.CompareAndSwap(old, count); old = peak.Load() {
		}
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprintf(w, `<body>%s</body>`, r.URL.Path)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan define.BodyParsed, 1)
	go func() {
		done <- GetDataAndConfigBySSRContext(ctx, define.JavaScriptConfig{URL: server.URL, ListContainer: "article", Title: "h2", DisableCache: true, ContentHook: define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: "body"}})
	}()
	for range detailWorkers {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatal("details did not run concurrently")
		}
	}
	close(release)
	got := <-done
	if got.Code != 0 || len(got.Body) != 8 || peak.Load() > detailWorkers {
		t.Fatalf("result=%#v peak=%d", got, peak.Load())
	}
	for i, item := range got.Body {
		if item.Title != fmt.Sprint(i) || item.Content != fmt.Sprintf("/detail/%d", i) {
			t.Fatalf("order changed: %#v", got.Body)
		}
	}
	if len(detailSlots) != 0 {
		t.Fatal("detail slots leaked")
	}
}

func TestSSRCookiesAndProxyReachPaginationAndDetails(t *testing.T) {
	logger.Initialize()
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Host != "feed.invalid" || r.Header.Get("Cookie") != "session=private" {
			t.Errorf("options lost: %s cookie=%q", r.URL, r.Header.Get("Cookie"))
		}
		if strings.HasPrefix(r.URL.Path, "/detail/") {
			fmt.Fprint(w, "<body>full</body>")
			return
		}
		fmt.Fprintf(w, `<article><h2>%s</h2><a href="/detail%s">read</a></article>`, r.URL.Path, r.URL.Path)
		if r.URL.Path == "/list" {
			fmt.Fprint(w, `<a class="page" href="/page2">next</a>`)
		}
	}))
	defer proxy.Close()
	got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: "http://feed.invalid/list", Proxy: proxy.URL, Cookies: "session=private", ListContainer: "article", Title: "h2", Pager: ".page", PagerLimit: 2, DisableCache: true, ContentHook: define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: "body"}})
	if got.Code != 0 || len(got.Body) != 2 || requests.Load() != 4 {
		t.Fatalf("%#v requests=%d", got, requests.Load())
	}
}

func TestDetailFailureCancelsSiblingRequests(t *testing.T) {
	logger.Initialize()
	entered := make(chan struct{}, detailWorkers)
	release := make(chan struct{})
	var canceled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			for i := 0; i < detailWorkers; i++ {
				fmt.Fprintf(w, `<article><h2>%d</h2><a href="/detail/%d">read</a></article>`, i, i)
			}
			return
		}
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		if r.URL.Path == "/detail/0" {
			w.WriteHeader(503)
			return
		}
		<-r.Context().Done()
		canceled.Add(1)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan define.BodyParsed, 1)
	go func() {
		done <- GetDataAndConfigBySSRContext(ctx, define.JavaScriptConfig{URL: server.URL, ListContainer: "article", Title: "h2", DisableCache: true, ContentHook: define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: "body"}})
	}()
	for range detailWorkers {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			close(release)
			cancel()
			t.Fatal("details not started")
		}
	}
	close(release)
	got := <-done
	if got.Code == 0 || len(got.Body) != 0 {
		t.Fatalf("partial response: %#v", got)
	}
	if len(detailSlots) != 0 {
		t.Fatal("slots leaked")
	}
	until := time.Now().Add(time.Second)
	for canceled.Load() != detailWorkers-1 {
		if time.Now().After(until) {
			t.Fatalf("siblings not canceled: %d", canceled.Load())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestInlineMetadataSkipsOverriddenHooks(t *testing.T) {
	logger.Initialize()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	hook := define.ConfigHook{Action: define.ConfigHookReadLink, URL: "a", Object: "body"}
	config := define.JavaScriptConfig{URL: server.URL, ListContainer: "article", Title: "h2", DateTime: "time", Category: ".category", Description: ".summary", DateTimeHook: hook, CategoryHook: hook, DescriptionHook: hook, DisableCache: true}
	got := ParseDataAndConfigBySSR(config, define.RemoteBodySanitized{}, `<article><h2>Title</h2><a href="/detail">read</a><time>today</time><p class="category">News</p><p class="summary">Summary</p></article>`)
	if got.Code != 0 || len(got.Body) != 1 || requests.Load() != 0 {
		t.Fatalf("overridden hook executed: %#v requests=%d", got, requests.Load())
	}
}
