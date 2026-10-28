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

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func wireJSON(t *testing.T, items ...csrWireItem) string {
	t.Helper()
	body, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
func TestCSRPostprocessNormalizesAndFillsNativeHooks(t *testing.T) {
	logger.Initialize()
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		cookie, err := r.Cookie("token")
		if err != nil || cookie.Value != "secret" {
			t.Errorf("scoped cookie missing: %v %v", cookie, err)
		}
		switch r.URL.Path {
		case "/date":
			fmt.Fprint(w, `<body><span>2026-10-29</span></body>`)
		case "/category":
			fmt.Fprint(w, `<body><span>技术</span></body>`)
		case "/description":
			fmt.Fprint(w, `<body><span>Summary</span></body>`)
		case "/content":
			fmt.Fprint(w, `<body><span>Article</span></body>`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	hook := func(url string) define.ConfigHook {
		return define.ConfigHook{Action: "readlink", URL: url, Object: "span"}
	}
	config := define.JavaScriptConfig{URL: upstream.URL + "/list", Cookies: "token=secret", DisableCache: true, IdByRegexp: `^/post/(\d+)$`, DateTimeHook: hook(".date"), CategoryHook: hook(".category"), DescriptionHook: hook(".description"), ContentHook: hook("a")}
	body := wireJSON(t, csrWireItem{InfoItem: define.InfoItem{Title: "Title", Link: "/post/42"}, Native: true, Hooks: &csrHookLinks{DateTime: "/date", Category: "/category", Description: "/description", Content: "/content"}})
	result := processCSRItemsContext(t.Context(), config, body)
	if result.Code != 0 || len(result.Body) != 1 {
		t.Fatalf("%+v", result)
	}
	item := result.Body[0]
	if item.ID != "42" || item.Link != upstream.URL+"/post/42" || item.Date != "2026-10-29" || item.Category != "技术" || item.Description != "Summary" || item.Content != "Article" || requests.Load() != 4 {
		t.Fatalf("item=%+v requests=%d", item, requests.Load())
	}
}

func TestCSRPostprocessPreservesCustomFieldsAndSupportsSelfLink(t *testing.T) {
	logger.Initialize()
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `<html><body>Fetched</body></html>`)
	}))
	defer upstream.Close()
	hook := define.ConfigHook{Action: "readlink", URL: ".unknown", Object: "body"}
	config := define.JavaScriptConfig{URL: upstream.URL, DisableCache: true, IdByRegexp: `/post/(\d+)`, DateTimeHook: hook, CategoryHook: hook, DescriptionHook: hook, ContentHook: hook}
	custom := define.InfoItem{ID: "custom-id", Title: "Title", Link: "/post/42", Date: "2026-10-29", Category: "Custom", Description: "Summary", Content: "Explicit"}
	result := processCSRItemsContext(t.Context(), config, wireJSON(t, csrWireItem{InfoItem: custom}))
	if result.Code != 0 || len(result.Body) != 1 || requests.Load() != 0 {
		t.Fatalf("%+v requests=%d", result, requests.Load())
	}
	item := result.Body[0]
	custom.Link = upstream.URL + "/post/42"
	if item != custom {
		t.Fatalf("custom output overwritten: %+v", item)
	}
	config = define.JavaScriptConfig{URL: upstream.URL, DisableCache: true, ContentHook: define.ConfigHook{Action: "readlink"}}
	result = processCSRItemsContext(t.Context(), config, wireJSON(t, csrWireItem{InfoItem: define.InfoItem{Link: "/post/42"}}))
	if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Content != "Fetched" || requests.Load() != 1 {
		t.Fatalf("self link: %+v", result)
	}
	config.ContentHook.URL = "literal:/post/43"
	result = processCSRItemsContext(t.Context(), config, wireJSON(t, csrWireItem{InfoItem: define.InfoItem{Link: "/post/42"}}))
	if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Content != "Fetched" || requests.Load() != 2 {
		t.Fatalf("literal hook URL: %+v", result)
	}
}

func TestCSRPostprocessRejectsFailedOrMissingHooksWithoutPartialData(t *testing.T) {
	logger.Initialize()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer upstream.Close()
	config := define.JavaScriptConfig{URL: upstream.URL, DisableCache: true, ContentHook: define.ConfigHook{Action: "readlink", URL: "a", Object: "body"}}
	for name, item := range map[string]csrWireItem{
		"native missing":              {InfoItem: define.InfoItem{Link: "/post/42"}, Native: true, Hooks: &csrHookLinks{}},
		"custom selector unavailable": {InfoItem: define.InfoItem{Link: "/post/42"}},
		"upstream failure":            {InfoItem: define.InfoItem{Link: "/post/42"}, Native: true, Hooks: &csrHookLinks{Content: "/fail"}},
	} {
		t.Run(name, func(t *testing.T) {
			result := processCSRItemsContext(t.Context(), config, wireJSON(t, item))
			if result.Code == 0 || result.Body != nil {
				t.Fatalf("partial content escaped: %+v", result)
			}
		})
	}
}

func TestCSRPostprocessInlineOverridesAndIDNoMatch(t *testing.T) {
	config := define.JavaScriptConfig{URL: "https://example.org/list", DateTime: ".inline", DateTimeHook: define.ConfigHook{Action: "readlink", URL: "a"}, IdByRegexp: `^/different/(\d+)$`}
	result := processCSRItemsContext(t.Context(), config, wireJSON(t, csrWireItem{InfoItem: define.InfoItem{Link: "/post/42"}, Native: true, Hooks: &csrHookLinks{}}))
	if result.Code != 0 || len(result.Body) != 1 || result.Body[0].ID != "" || result.Body[0].Link != "https://example.org/post/42" {
		t.Fatalf("%+v", result)
	}
}

func TestCSRPostprocessSharedRawDataKeepsCallerCancellationIndependent(t *testing.T) {
	logger.Initialize()
	var requests atomic.Int32
	entered := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer upstream.Close()
	config := define.JavaScriptConfig{URL: upstream.URL, DisableCache: true, ContentHook: define.ConfigHook{Action: "readlink", URL: "a", Object: "body"}}
	raw := wireJSON(t, csrWireItem{InfoItem: define.InfoItem{Link: "/post/42"}, Native: true, Hooks: &csrHookLinks{Content: "/detail"}})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan define.BodyParsed, 1)
	go func() { done <- processCSRItemsContext(ctx, config, raw) }()
	<-entered
	cancel()
	select {
	case result := <-done:
		if result.Code == 0 || result.Body != nil {
			t.Fatalf("%+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("detail cancellation left running")
	}
	// A separate waiter decodes the same cached/raw string into its own item.
	// Its explicit data must not have been changed by the canceled caller.
	noHook := config
	noHook.ContentHook = define.ConfigHook{}
	result := processCSRItemsContext(t.Context(), noHook, raw)
	if result.Code != 0 || len(result.Body) != 1 || result.Body[0].Content != "" || requests.Load() != 1 {
		t.Fatalf("independent decode: %+v", result)
	}
}

func TestCSRPostprocessLimitsAndMalformedData(t *testing.T) {
	config := define.JavaScriptConfig{URL: "https://example.org"}
	for _, data := range []string{"", `{}`, `[`, `[{}] trailing`, `null trailing`, strings.Repeat("x", limits.MaxPageBytes+1), `[{"link":"%"}]`} {
		result := processCSRItemsContext(t.Context(), config, data)
		if result.Code == 0 || result.Body != nil {
			t.Fatalf("invalid %q accepted", data[:min(20, len(data))])
		}
	}
	if result := processCSRItemsContext(t.Context(), config, "null"); result.Code != 0 || len(result.Body) != 0 {
		t.Fatalf("legacy null: %+v", result)
	}
	if result := processCSRItemsContext(t.Context(), config, "[]"); result.Code != 0 || len(result.Body) != 0 {
		t.Fatalf("empty array: %+v", result)
	}
	tooMany := make([]csrWireItem, limits.MaxItems+1)
	if result := processCSRItemsContext(t.Context(), config, wireJSON(t, tooMany...)); result.Code == 0 {
		t.Fatal("item limit bypassed")
	}
	large := wireJSON(t, csrWireItem{InfoItem: define.InfoItem{Link: "/" + strings.Repeat("x", 6<<20)}})
	config.IdByRegexp = `^/(.*)$`
	if result := processCSRItemsContext(t.Context(), config, large); result.Code == 0 {
		t.Fatal("expanded ID/link budget bypassed")
	}
	config.IdByRegexp = "["
	if result := processCSRItemsContext(t.Context(), config, "[]"); result.Code == 0 {
		t.Fatal("bad regexp accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result := processCSRItemsContext(ctx, config, "[]"); result.Code == 0 {
		t.Fatal("canceled decode accepted")
	}
}

func TestCSRPostprocessReadsRawBrowserCachePerCaller(t *testing.T) {
	logger.Initialize()
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	defer cache.Close()
	ctx := cacher.WithCache(t.Context(), cache)
	raw := wireJSON(t, csrWireItem{InfoItem: define.InfoItem{Link: "/post/42"}})
	if err := cacher.SetWithTTL(ctx, "raw-browser", raw, time.Minute); err != nil {
		t.Fatal(err)
	}
	config := define.JavaScriptConfig{URL: "https://example.org", IdByRegexp: `^/post/(\d+)$`}
	cached, found, err := cacher.GetWithinLimit(ctx, "raw-browser", limits.MaxPageBytes)
	if err != nil || !found {
		t.Fatal(err)
	}
	for range 2 {
		result := processCSRItemsContext(ctx, config, cached)
		if result.Code != 0 || result.Body[0].ID != "42" || result.Body[0].Link != "https://example.org/post/42" {
			t.Fatalf("%+v", result)
		}
		result.Body[0].ID = "mutated"
	}
}
