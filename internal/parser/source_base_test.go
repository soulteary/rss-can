package parser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
)

func TestSSRRedirectBasePaginationDetailsAndCache(t *testing.T) {
	logger.Initialize()
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/news/index.html", 302)
		case "/news/index.html":
			fmt.Fprint(w, `<html><head><base href="../articles/"></head><body><article><h2>First</h2><a href="post/42">read</a></article><a class="pager" href="page2.html">next</a></body></html>`)
		case "/articles/page2.html":
			fmt.Fprint(w, `<article><h2>Second</h2><a href="post/43">read</a></article>`)
		case "/articles/post/42":
			fmt.Fprint(w, `<body>Detail 42</body>`)
		case "/articles/post/43":
			fmt.Fprint(w, `<body>Detail 43</body>`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	defer cache.Close()
	ctx := network.WithClient(cacher.WithCache(t.Context(), cache), network.NewClient(network.Config{UserAgent: "base-test"}))
	config := define.JavaScriptConfig{URL: upstream.URL + "/start", Mode: "ssr", Title: "h2", Link: "a", ListContainer: "article", Pager: ".pager", PagerLimit: 2, Expire: 60, ContentHook: define.ConfigHook{Action: "readlink", URL: "a", Object: "body"}}
	for range 2 {
		result := GetDataAndConfigBySSRContext(ctx, config)
		if result.Code != 0 || len(result.Body) != 2 {
			t.Fatalf("parse: %+v", result)
		}
		for i, id := range []string{"42", "43"} {
			if result.Body[i].Link != upstream.URL+"/articles/post/"+id || !strings.Contains(result.Body[i].Content, "Detail "+id) {
				t.Fatalf("bad base: %+v", result.Body)
			}
		}
	}
	if calls.Load() != 5 {
		t.Fatalf("cache lost source provenance or refetched: %d", calls.Load())
	}
}
func TestCSRDocumentBaseKeepsCookieOrigin(t *testing.T) {
	logger.Initialize()
	var cookie string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { cookie = r.Header.Get("Cookie"); fmt.Fprint(w, "detail") }))
	defer target.Close()
	// Different hostnames, not merely ports, are needed for host-only cookies.
	base := strings.Replace(target.URL, "127.0.0.1", "localhost", 1) + "/news/"
	config := define.JavaScriptConfig{URL: "http://127.0.0.1:1/source", Mode: "csr", ListContainer: "article", Title: "h2", Link: "a", Cookies: "secret=value", DisableCache: true, ContentHook: define.ConfigHook{Action: "readlink", URL: "", Object: "body"}}
	result := processCSRItemsContext(t.Context(), config, `[{"title":"Title","link":"post/42"}]`, base)
	if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Link != base+"post/42" || result.Body[0].Content != "detail" {
		t.Fatalf("bad CSR base: %+v", result)
	}
	if cookie != "" {
		t.Fatalf("cookie rebound to redirect target: %q", cookie)
	}
}
func TestSSRBaseMetadataAndInvalidHTMLBase(t *testing.T) {
	config := define.JavaScriptConfig{URL: "https://original.example/start", Title: "h2", Link: "a", ListContainer: "article"}
	for _, tc := range []struct {
		doc  define.RemoteBodySanitized
		want string
	}{
		{define.RemoteBodySanitized{FinalURL: "https://target.example/news/", Body: `<article><h2>Title</h2><a href="post">read</a></article>`}, "https://target.example/news/post"},
		{define.RemoteBodySanitized{FinalURL: "https://target.example/news/", BaseURL: "https://base.example/articles/", Body: `<base href="wrong/"><article><h2>Title</h2><a href="post">read</a></article>`}, "https://base.example/articles/post"},
		{define.RemoteBodySanitized{FinalURL: "https://target.example/news/", Body: `<base href="javascript:bad"><article><h2>Title</h2><a href="post">read</a></article>`}, "https://target.example/news/post"},
		{define.RemoteBodySanitized{FinalURL: "https://target.example/news/", Body: `<base href="javascript:bad"><base href="https://wrong.example/"><article><h2>Title</h2><a href="post">read</a></article>`}, "https://target.example/news/post"},
		{define.RemoteBodySanitized{FinalURL: "https://target.example/news/", Body: `<base target="_blank"><base href="../articles/"><article><h2>Title</h2><a href="post">read</a></article>`}, "https://target.example/articles/post"},
	} {
		result := ParseDataAndConfigBySSRContext(t.Context(), config, tc.doc, "")
		if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Link != tc.want {
			t.Fatalf("metadata base: %+v want%s", result, tc.want)
		}
	}
}

func TestSSRDistinctBaseRootPagerIsFetchedOnce(t *testing.T) {
	logger.Initialize()
	var basePages, detailPages atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/news/index.html", 302)
		case "/news/index.html":
			fmt.Fprint(w, `<head><base href="../articles/"></head><body><article><h2>First</h2><a href="post/42">read</a></article><a class="pager" href="./">next</a><a class="pager" href="./#same">same</a><a class="pager" href="/news/index.html">current</a><a class="pager" href="/start">original</a></body>`)
		case "/articles/":
			basePages.Add(1)
			fmt.Fprint(w, `<article><h2>Second</h2><a href="post/43">read</a></article>`)
		case "/articles/post/42":
			detailPages.Add(1)
			fmt.Fprint(w, `<body>Detail 42</body>`)
		case "/articles/post/43":
			detailPages.Add(1)
			fmt.Fprint(w, `<body>Detail 43</body>`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	config := define.JavaScriptConfig{URL: upstream.URL + "/start", Mode: "ssr", Title: "h2", Link: "a", ListContainer: "article", Pager: ".pager", PagerLimit: 5, DisableCache: true, ContentHook: define.ConfigHook{Action: "readlink", URL: "a", Object: "body"}}
	result := GetDataAndConfigBySSRContext(t.Context(), config)
	if result.Code != 0 || len(result.Body) != 2 {
		t.Fatalf("base root pager skipped or current page duplicated: %+v", result)
	}
	if basePages.Load() != 1 || detailPages.Load() != 2 {
		t.Fatalf("base pages=%d details=%d", basePages.Load(), detailPages.Load())
	}
	for i, id := range []string{"42", "43"} {
		if result.Body[i].Link != upstream.URL+"/articles/post/"+id || result.Body[i].Content != "Detail "+id {
			t.Fatalf("wrong link/detail base: %+v", result.Body[i])
		}
	}
}

func TestDocumentBaseIgnoresTemplateContents(t *testing.T) {
	final := "https://target.example/news/"
	for _, tc := range []struct{ body, want string }{
		{`<template><base href="https://wrong.example/"></template>`, final},
		{`<template><template><base href="https://wrong.example/"></template><base href="https://wrong.example/"></template><base href="../articles/">`, "https://target.example/articles/"},
		{`<template/><base href="https://wrong.example/"></template><base href="../articles/">`, "https://target.example/articles/"},
		{`<template><base href="https://wrong.example/">`, final},
	} {
		if got := documentBaseFromHTML(final, tc.body); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.body, got, tc.want)
		}
	}
}
