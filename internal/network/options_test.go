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
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestRequestOptionsCookiesRedirectAndCacheIsolation(t *testing.T) {
	logger.Initialize()
	redis, memory := define.REDIS, define.IN_MEMORY_CACHE
	define.REDIS, define.IN_MEMORY_CACHE = false, true
	defer func() { define.REDIS, define.IN_MEMORY_CACHE = redis, memory }()
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprintf(w, `<body>%s</body>`, r.Header.Get("Cookie"))
	}))
	defer upstream.Close()
	for _, cookie := range []string{"session=one", "session=two", "session=one"} {
		opts := RequestOptions{CookieURL: upstream.URL + "/nested/list", Cookies: cookie}
		got := GetRemoteDocumentContext(context.Background(), upstream.URL+"/detail", "utf-8", 60, false, opts)
		if got.Code != 0 || !strings.Contains(got.Body, cookie) {
			t.Fatalf("cookie lost or cache crossed: %#v", got)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("cache requests: %d", requests.Load())
	}
	otherHost := strings.Replace(upstream.URL, "127.0.0.1", "localhost", 1)
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, otherHost, http.StatusFound) }))
	defer redirect.Close()
	opts := RequestOptions{CookieURL: redirect.URL, Cookies: "session=private"}
	got := GetRemoteDocumentContext(context.Background(), redirect.URL, "utf-8", 1, true, opts)
	if got.Code != 0 || strings.Contains(got.Body, "private") {
		t.Fatalf("cross-host redirect leaked cookie: %#v", got)
	}
	got = GetRemoteDocumentContext(context.Background(), otherHost, "utf-8", 1, true, opts)
	if got.Code != 0 || strings.Contains(got.Body, "private") {
		t.Fatalf("cross-host detail leaked cookie: %#v", got)
	}
}

func TestExplicitProxyOverridesEnvironmentAndSeparatesCache(t *testing.T) {
	logger.Initialize()
	redis, memory := define.REDIS, define.IN_MEMORY_CACHE
	define.REDIS, define.IN_MEMORY_CACHE = false, true
	defer func() { define.REDIS, define.IN_MEMORY_CACHE = redis, memory }()
	for _, text := range []string{"proxy-one", "proxy-two"} {
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Host != "feed.invalid" || r.Header.Get("Cookie") != "session=proxy" {
				t.Errorf("bad proxy request: %s %s", r.URL, r.Header.Get("Cookie"))
			}
			fmt.Fprint(w, text)
		}))
		opts := RequestOptions{Proxy: proxy.URL, CookieURL: "http://feed.invalid", Cookies: "session=proxy"}
		got := GetRemoteDocumentContext(context.Background(), "http://feed.invalid/list", "utf-8", 60, false, opts)
		proxy.Close()
		if got.Code != 0 || got.Body != text {
			t.Fatalf("proxy or cache failed: %#v", got)
		}
	}
	for _, raw := range []string{"ftp://host:21", "http://", "http://host/bad", "://bad"} {
		if _, err := NormalizeProxy(raw); err == nil {
			t.Fatalf("accepted proxy %q", raw)
		}
	}
	if got, err := NormalizeProxy("127.0.0.1:3128"); err != nil || got != "http://127.0.0.1:3128" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestStrictMarkdownPreservesFailure(t *testing.T) {
	logger.Initialize()
	for _, tc := range []struct {
		name, body, selector string
		status               int
		code                 define.ErrorCode
	}{
		{"upstream", "", "body", 503, define.ERROR_CODE_API_NOT_READY},
		{"missing", "<body>text</body>", ".missing", 200, define.ERROR_CODE_PARSE_CONTENT_FAILED},
		{"empty response", "", "body", 200, define.ERROR_CODE_PARSE_CONTENT_FAILED},
		{"empty element", "<body><div></div></body>", "div", 200, define.ERROR_CODE_NULL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			got := GetRemoteMarkdownContext(context.Background(), server.URL, tc.selector, "utf-8", 1, true)
			if got.Code != tc.code {
				t.Fatalf("%#v", got)
			}
		})
	}
}
