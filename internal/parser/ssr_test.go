package parser

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestSSRPropagatesUpstreamFailures(t *testing.T) {
	logger.Initialize()
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer upstream.Close()
			got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: upstream.URL, DisableCache: true})
			if got.Code == define.ERROR_CODE_NULL || got.Status == "" {
				t.Fatalf("upstream %d treated as success: %#v", status, got)
			}
		})
	}
	got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: "://invalid", DisableCache: true})
	if got.Code != define.ERROR_CODE_INIT_NETWORK_FAILED {
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
	got := GetDataAndConfigBySSR(define.JavaScriptConfig{URL: upstream.URL, Pager: ".page", PagerLimit: 2, DisableCache: true})
	if got.Code != define.ERROR_CODE_API_NOT_READY {
		t.Fatalf("pagination error lost: %#v", got)
	}
}
