package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func validBrowserDocument() browserDocument {
	return browserDocument{Version: 1, Body: "[]", FinalURL: "https://example.org/news/", BaseURL: "https://example.org/news/", HTTPStatus: 200}
}
func TestBrowserDocumentRecordValidation(t *testing.T) {
	valid := validBrowserDocument()
	encoded, err := encodeBrowserDocument(valid)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeBrowserDocument(encoded)
	if err != nil || decoded != valid {
		t.Fatalf("%+v %v", decoded, err)
	}
	for _, changed := range []browserDocument{
		{Version: 1, Body: "[]", FinalURL: valid.FinalURL, BaseURL: valid.BaseURL, HTTPStatus: 503},
		{Version: 1, Body: "[]", FinalURL: "", BaseURL: valid.BaseURL, HTTPStatus: 200},
		{Version: 1, Body: "[]", FinalURL: valid.FinalURL, BaseURL: "file:///tmp", HTTPStatus: 200},
		{Version: 2, Body: "[]", FinalURL: valid.FinalURL, BaseURL: valid.BaseURL, HTTPStatus: 200},
		{Version: 1, Body: strings.Repeat("x", limits.MaxPageBytes+1), FinalURL: valid.FinalURL, BaseURL: valid.BaseURL, HTTPStatus: 200},
	} {
		if _, err := encodeBrowserDocument(changed); err == nil {
			t.Fatal("invalid fetched document encoded")
		}
		raw, _ := json.Marshal(changed)
		if err := validateBrowserDocument(string(raw)); err == nil {
			t.Fatal("invalid cache document accepted")
		}
	}
	if _, err := decodeBrowserDocument("not-json"); err == nil {
		t.Fatal("malformed cache record accepted")
	}
	if validateBrowserURL("https://example.org/" + strings.Repeat("x", limits.MaxDocumentURLBytes)) {
		t.Fatal("oversize URL accepted")
	}
	// JSON escaping does not reduce the documented decoded-page budget.
	valid.Body = strings.Repeat("\"", limits.MaxPageBytes)
	encoded, err = encodeBrowserDocument(valid)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = decodeBrowserDocument(encoded)
	if err != nil || decoded.Body != valid.Body {
		t.Fatal("escaped boundary document rejected")
	}
}

func TestBrowserInvalidEnvelopeIsRefetchedAndFailureNotCached(t *testing.T) {
	logger.Initialize()
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	defer cache.Close()
	ctx := cacher.WithCache(t.Context(), cache)
	invalid := validBrowserDocument()
	invalid.HTTPStatus = 503
	raw, _ := json.Marshal(invalid)
	if err := cacher.SetWithTTL(ctx, "browser-invalid", string(raw), time.Minute); err != nil {
		t.Fatal(err)
	}
	valid, err := encodeBrowserDocument(validBrowserDocument())
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	fetch := func(context.Context) (string, error) { calls++; return valid, nil }
	for range 2 {
		result, err := browserData(ctx, "browser-invalid", true, time.Second, 60, fetch, validateBrowserDocument)
		if err != nil || result != valid {
			t.Fatalf("%q %v", result, err)
		}
	}
	if calls != 1 {
		t.Fatalf("invalid cache wasn't replaced: %d", calls)
	}
	for range 2 {
		_, err := browserData(ctx, "browser-failure", true, time.Second, 60, func(context.Context) (string, error) { return string(raw), nil }, validateBrowserDocument)
		if err == nil {
			t.Fatal("failed document accepted")
		}
	}
	if _, found, err := cacher.GetWithStatus(ctx, "browser-failure"); err != nil || found {
		t.Fatal("failure entered cache")
	}
}

func TestMainDocumentResponsesIgnoreOtherFramesAndTypes(t *testing.T) {
	r := &mainDocumentResponses{frame: "main", statuses: make(map[cdp.LoaderID]int), changed: make(chan struct{}, 1)}
	for _, event := range []cdpnetwork.EventResponseReceived{
		{FrameID: "child", LoaderID: "bad-child", Type: cdpnetwork.ResourceTypeDocument, Response: &cdpnetwork.Response{Status: 503}},
		{FrameID: "main", LoaderID: "script", Type: cdpnetwork.ResourceTypeScript, Response: &cdpnetwork.Response{Status: 503}},
		{FrameID: "main", LoaderID: "nil", Type: cdpnetwork.ResourceTypeDocument},
	} {
		r.record(&event)
	}
	if len(r.statuses) != 0 {
		t.Fatal("subresource status poisoned document")
	}
	for i := 0; i < 20; i++ {
		r.record(&cdpnetwork.EventResponseReceived{FrameID: "main", LoaderID: cdp.LoaderID(fmt.Sprint(i)), Type: cdpnetwork.ResourceTypeDocument, Response: &cdpnetwork.Response{Status: 200}})
	}
	if len(r.statuses) != 16 || r.statuses["19"] != 200 {
		t.Fatal("navigation tracker unbounded or lost latest loader")
	}
}

func TestLiveBrowserMainDocumentStatusAndMetadata(t *testing.T) {
	address := readinessBrowser(t)
	var failures atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect-good":
			http.Redirect(w, r, "/news/", 302)
			return
		case "/redirect-bad":
			http.Redirect(w, r, "/503", 302)
			return
		case "/404":
			w.WriteHeader(404)
		case "/503":
			failures.Add(1)
			w.WriteHeader(503)
		case "/empty503":
			w.WriteHeader(503)
			fmt.Fprint(w, `<html><body><main id="loaded">Maintenance</main></body></html>`)
			return
		case "/bad-script":
			w.WriteHeader(503)
			fmt.Fprint(w, "bad script")
			return
		case "/bad-frame":
			w.WriteHeader(503)
			fmt.Fprint(w, "bad iframe")
			return
		case "/client-redirect":
			fmt.Fprint(w, `<html><body><script>setTimeout(()=>location.href='/empty503',20)</script></body></html>`)
			return
		}
		fmt.Fprint(w, `<html><head><base href="/articles/"></head><body><main id="loaded"><article><h2>Title</h2><a href="post/42">read</a></article></main><script src="/bad-script"></script><iframe src="/bad-frame"></iframe></body></html>`)
	}))
	defer upstream.Close()
	for _, mix := range []bool{false, true} {
		for _, path := range []string{"/404", "/503", "/empty503", "/redirect-bad", "/client-redirect"} {
			config := readinessConfig(upstream.URL+path, "csr", "#loaded")
			result := ParsePageByBrowserContext(t.Context(), config, address, "", mix)
			if result.Code == 0 || len(result.Body) != 0 || !strings.Contains(result.Status, "HTTP") {
				t.Fatalf("failed main document accepted mix=%t path=%s: %+v", mix, path, result)
			}
		}
		config := readinessConfig(upstream.URL+"/redirect-good", "csr", "#loaded")
		work, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		raw, err := fetchBrowserData(work, config, address, "", mix)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		document, err := decodeBrowserDocument(raw)
		if err != nil || document.HTTPStatus != 200 || document.FinalURL != upstream.URL+"/news/" || document.BaseURL != upstream.URL+"/articles/" {
			t.Fatalf("metadata lost: %+v %v", document, err)
		}
		result := ParsePageByBrowserContext(t.Context(), config, address, "", mix)
		if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Link != upstream.URL+"/articles/post/42" {
			t.Fatalf("successful document poisoned by subresource or lost base: %+v", result)
		}
	}
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	defer cache.Close()
	ctx := cacher.WithCache(t.Context(), cache)
	config := readinessConfig(upstream.URL+"/503", "csr", "body")
	config.DisableCache = false
	before := failures.Load()
	for range 2 {
		if result := ParsePageByBrowserContext(ctx, config, address, "", false); result.Code == 0 {
			t.Fatal("HTTP503 accepted")
		}
	}
	if failures.Load() != before+2 {
		t.Fatal("failed document was cached")
	}
}
