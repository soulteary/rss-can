package network

import (
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
)

func TestDocumentRedirectMetadataSurvivesCache(t *testing.T) {
	ctx, ua := boundedCacheContext(t)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/news" {
			http.Redirect(w, r, "/news/", 302)
			return
		}
		fmt.Fprint(w, `<article>body</article>`)
	}))
	defer upstream.Close()
	target := upstream.URL + "/news"
	for range 2 {
		document := GetRemoteDocumentContext(ctx, target, "", 60, false)
		if document.Code != 0 || document.FinalURL != upstream.URL+"/news/" || document.HTTPStatus != 200 || document.Body != "<article>body</article>" {
			t.Fatalf("metadata lost: %+v", document)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("cache refetched: %d", calls.Load())
	}
	key := seededDocumentKey(target, ua)
	value, found, err := cacher.GetWithStatus(ctx, key)
	if err != nil || !found {
		t.Fatal(err)
	}
	var stored define.RemoteBodySanitized
	if json.Unmarshal([]byte(value), &stored) != nil || stored.FinalURL != upstream.URL+"/news/" {
		t.Fatal("cache has no provenance")
	}
}
func TestInvalidDocumentCacheMetadataRefetches(t *testing.T) {
	ctx, ua := boundedCacheContext(t)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, "fresh") }))
	defer upstream.Close()
	key := seededDocumentKey(upstream.URL, ua)
	for _, value := range []string{`{"data":"old body","http_status":200}`, `{"data":"error","http_status":503,"final_url":"https://example.org"}`, `{"data":"wrong","http_status":200,"final_url":"file:///etc/passwd"}`, `{"data":"wrong","http_status":200,"final_url":"https://example.org","base_url":"file:///etc/passwd"}`, `{"data":"wrong","code":1,"http_status":200,"final_url":"https://example.org"}`} {
		if err := cacher.SetWithTTL(ctx, key, value, time.Minute); err != nil {
			t.Fatal(err)
		}
		result := GetRemoteDocumentContext(ctx, upstream.URL, "", 60, false)
		if result.Code != 0 || result.Body != "fresh" {
			t.Fatalf("invalid record used: %+v", result)
		}
	}
	if calls.Load() != 5 {
		t.Fatalf("bad metadata did not refetch: %d", calls.Load())
	}
}
func TestEscapedDocumentCacheEnvelopeUsesBodyBudget(t *testing.T) {
	ctx, _ := boundedCacheContext(t)
	// The escaped wire record may exceed the page limit while the decoded body
	// is valid. This must not reject the fetched document itself.
	body := strings.Repeat(`"`, MaxDocumentBytes)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer upstream.Close()
	result := GetRemoteDocumentContext(ctx, upstream.URL, "", 60, false)
	if result.Code != 0 || len(result.Body) != MaxDocumentBytes {
		t.Fatalf("valid body rejected: %d %d", result.Code, len(result.Body))
	}
	encoded, _ := json.Marshal(result)
	if len(encoded) <= MaxDocumentBytes || len(encoded) > limits.MaxDocumentCacheBytes {
		t.Fatalf("wrong record budget: %d", len(encoded))
	}
}
