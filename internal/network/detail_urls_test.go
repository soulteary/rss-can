package network

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/goquery"
)

func TestDetailMarkdownRedirectBaseAndCache(t *testing.T) {
	ctx, _ := boundedCacheContext(t)
	for _, useBase := range []bool{false, true} {
		t.Run(fmt.Sprint(useBase), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/start" {
					http.Redirect(w, r, "/news/detail/", http.StatusFound)
					return
				}
				if useBase {
					fmt.Fprint(w, `<head><template><base href="https://wrong.example/"></template><base target="_blank"><base href="../../assets/"></head>`)
				}
				fmt.Fprint(w, `<body><main><a href="next">next</a><img src="pic.png"><a href="#part">part</a><a href="//cdn.example/a">cdn</a></main></body>`)
			}))
			defer server.Close()
			target := server.URL + "/start"
			base := server.URL + "/news/detail/"
			if useBase {
				base = server.URL + "/assets/"
			}
			for pass := range 2 {
				if pass == 1 {
					server.Close()
				}
				result := GetRemoteMarkdownContext(ctx, target, "main", "", 60, false)
				if result.Code != 0 || result.FinalURL != server.URL+"/news/detail/" || result.BaseURL != base {
					t.Fatalf("provenance: %+v", result)
				}
				for _, want := range []string{"[next](" + base + "next)", "![](" + base + "pic.png)", "[part](" + base + "#part)", "[cdn](http://cdn.example/a)"} {
					if !strings.Contains(result.Body, want) {
						t.Errorf("pass %d missing %q in %q", pass, want, result.Body)
					}
				}
			}
			if calls.Load() != 2 {
				t.Fatalf("cache re-fetched: %d", calls.Load())
			}
		})
	}
}

func TestDetailBaseSelectsFirstActiveHref(t *testing.T) {
	final := "https://site.example/news/detail/"
	for _, test := range []struct{ name, body, metadata, want string }{
		{"no base", `<body>text</body>`, "", final},
		{"SVG base is ignored", `<svg><base href="https://wrong.example/"></base></svg>`, "", final},
		{"SVG base does not hide HTML base", `<svg><base href="https://wrong.example/"></base></svg><base href="/assets/">`, "", "https://site.example/assets/"},
		{"SVG template is not inert", `<svg><template><foreignObject><base href="/active/"></foreignObject></template></svg>`, "", "https://site.example/active/"},
		{"relative base", `<head><base href="../assets/"></head>`, "", "https://site.example/news/assets/"},
		{"no href", `<base target="_blank"><base href="/assets/">`, "", "https://site.example/assets/"},
		{"first invalid wins", `<base href="javascript:wrong"><base href="https://wrong.example/">`, "", final},
		{"malformed first wins", `<base href="http://%"><base href="https://wrong.example/">`, "", final},
		{"template base", `<template><base href="https://wrong.example/"></template><base href="/assets/">`, "", "https://site.example/assets/"},
		{"nested template", `<template><template><base href="https://wrong.example/"></template><base href="https://wrong.example/"></template>`, "", final},
		{"metadata override", `<base href="https://wrong.example/">`, "https://metadata.example/", "https://metadata.example/"},
		{"invalid metadata falls back", `<base href="/assets/">`, "file:///wrong", "https://site.example/assets/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.body))
			if err != nil {
				t.Fatal(err)
			}
			actual := detailDocumentBase(define.RemoteBodySanitized{FinalURL: final, BaseURL: test.metadata}, doc)
			if actual != test.want {
				t.Fatalf("got %q want %q", actual, test.want)
			}
		})
	}
}

func TestDetailURLsPreserveExplicitSchemesAndInvalidReferences(t *testing.T) {
	body := `<main><a href="mailto:a@example.org">mail</a><img src="data:image/png;base64,AA"><a href="javascript:void(0)">js</a><a href="https://else.example/a">absolute</a><a href="http://%">invalid</a><a href="">empty</a><template><a href="inert">inert</a></template></main>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	selection := doc.Find("main")
	before, _ := selection.Html()
	if err := normalizeDetailURLs(t.Context(), selection, "https://site.example/news/"); err != nil {
		t.Fatal(err)
	}
	after, _ := selection.Html()
	if before != after {
		t.Fatalf("explicit or inert URLs changed:\n%s\n%s", before, after)
	}
}

func TestDetailURLExpansionAndCancellation(t *testing.T) {
	body := `<main><a href="a">link</a><img src="pic"></main>`
	document := func() *goquery.Selection {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return doc.Find("main")
	}
	oversize := `<main><a href="a">link</a>` + strings.Repeat("x", limits.MaxFeedBytes-30) + `</main>`
	large, err := goquery.NewDocumentFromReader(strings.NewReader(oversize))
	if err != nil {
		t.Fatal(err)
	}
	if err := normalizeDetailURLs(t.Context(), large.Find("main"), "https://site.example/"); err == nil {
		t.Fatal("URL growth bypassed budget")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := normalizeDetailURLs(canceled, document(), "https://site.example/"); err != context.Canceled {
		t.Fatalf("canceled: %v", err)
	}
	if err := normalizeDetailURLs(t.Context(), document(), "http://%"); err == nil {
		t.Fatal("malformed base accepted")
	}
}

func TestDetailURLExpansionAccountsForHTMLEscaping(t *testing.T) {
	inner := `<a href="#part">link</a>`
	body := `<main>` + inner + strings.Repeat("x", limits.MaxFeedBytes-len(inner)-40) + `</main>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	// A base query expands its ampersands when rendered back to detached HTML.
	base := "https://site.example/?a=1&b=2&c=3&d=4"
	if err := normalizeDetailURLs(t.Context(), doc.Find("main"), base); err == nil {
		t.Fatal("escaped attribute expansion bypassed budget")
	}
}

func TestDetailHTMLSerializationStillObeysPageBudget(t *testing.T) {
	ctx, _ := boundedCacheContext(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<main>"+strings.Repeat("&", (limits.MaxPageBytes/5)+1)+"</main>")
	}))
	defer server.Close()
	result := GetRemoteMarkdownContext(ctx, server.URL, "main", "", 60, true)
	if result.Code != define.ERROR_CODE_PARSE_CONTENT_FAILED || result.Body != "" || !strings.Contains(result.Status, "HTML exceeds") {
		t.Fatalf("serialized HTML limit: code=%d status=%s body=%d", result.Code, result.Status, len(result.Body))
	}
}

func TestDetailSmallSelectionFromMaximumPage(t *testing.T) {
	ctx, _ := boundedCacheContext(t)
	prefix := `<html><body><aside>`
	suffix := `</aside><main href="ignored" src="ignored"><a href="next">next</a><img src="pic.png"></main></body></html>`
	body := prefix + strings.Repeat("x", limits.MaxPageBytes-len(prefix)-len(suffix)) + suffix
	if len(body) != limits.MaxPageBytes {
		t.Fatal("fixture must be exactly maximum upstream size")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer server.Close()
	result := GetRemoteMarkdownContext(ctx, server.URL+"/news/", "main", "", 60, true)
	if result.Code != 0 || !strings.Contains(result.Body, "[next]("+server.URL+"/news/next)") || !strings.Contains(result.Body, "![]("+server.URL+"/news/pic.png)") {
		t.Fatalf("small selected content failed: code=%d status=%s body=%q", result.Code, result.Status, result.Body)
	}
}

func TestDetailNormalizationDoesNotChargeSelectedRootAttributes(t *testing.T) {
	// These root attributes are absent from selection.Html, so neither the
	// rewriting nor its expansion budget should include them.
	inner := strings.Repeat("x", limits.MaxPageBytes)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<main href="a" src="b">` + inner + `</main>`))
	if err != nil {
		t.Fatal(err)
	}
	selection := doc.Find("main")
	if err := normalizeDetailURLs(t.Context(), selection, "https://site.example/news/"); err != nil {
		t.Fatal(err)
	}
	if href, _ := selection.Attr("href"); href != "a" {
		t.Fatalf("selected root href changed to %q", href)
	}
	if src, _ := selection.Attr("src"); src != "b" {
		t.Fatalf("selected root src changed to %q", src)
	}
}
