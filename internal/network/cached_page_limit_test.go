package network

import (
	"context"
	"crypto/sha256"
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
	"github.com/soulteary/RSS-Can/internal/logger"
)

func boundedCacheContext(t *testing.T) (context.Context, string) {
	t.Helper()
	logger.Initialize()
	cache := cacher.New(cacher.Config{MemoryEnabled: true})
	t.Cleanup(func() { cache.Close() })
	ua := "cached-page-limit"
	return WithClient(cacher.WithCache(t.Context(), cache), NewClient(Config{UserAgent: ua})), ua
}
func seededDocumentKey(url, ua string) string {
	return documentKey(url, "", 60, RequestOptions{Proxy: ""}) + fmt.Sprintf(":ua:%x", sha256.Sum256([]byte(ua)))
}
func TestCachedDocumentOversizeRefetchesAndReplaces(t *testing.T) {
	ctx, ua := boundedCacheContext(t)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, "<p>fresh</p>") }))
	defer upstream.Close()
	key := seededDocumentKey(upstream.URL, ua)
	if err := cacher.SetWithTTL(ctx, key, strings.Repeat("x", MaxDocumentBytes+1), time.Minute); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		got := GetRemoteDocumentContext(ctx, upstream.URL, "", 60, false)
		if got.Code != define.ERROR_CODE_NULL || got.Body != "<p>fresh</p>" {
			t.Fatalf("cached oversize returned: %d %d", got.Code, len(got.Body))
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("fresh fetches=%d", calls.Load())
	}
	if value, found, err := cacher.GetWithStatus(ctx, key); err != nil || !found {
		t.Fatalf("replacement %q %v %v", value, found, err)
	} else {
		var record define.RemoteBodySanitized
		if json.Unmarshal([]byte(value), &record) != nil || record.Body != "<p>fresh</p>" || record.FinalURL != upstream.URL {
			t.Fatalf("replacement record %s", value)
		}
	}
}
func TestCachedDocumentOversizeDoesNotHideUpstreamFailure(t *testing.T) {
	ctx, ua := boundedCacheContext(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer upstream.Close()
	key := seededDocumentKey(upstream.URL, ua)
	if err := cacher.SetWithTTL(ctx, key, strings.Repeat("x", MaxDocumentBytes+1), time.Minute); err != nil {
		t.Fatal(err)
	}
	got := GetRemoteDocumentContext(ctx, upstream.URL, "", 60, false)
	if got.Code == define.ERROR_CODE_NULL || got.Body != "" {
		t.Fatalf("invalid partial document returned: %d %d", got.Code, len(got.Body))
	}
	if _, found, _ := cacher.GetWithStatus(ctx, key); found {
		t.Fatal("invalid cached document survived failed refetch")
	}
}
func TestCachedDocumentBoundaryAndCancellation(t *testing.T) {
	ctx, ua := boundedCacheContext(t)
	url := "https://unused.invalid"
	key := seededDocumentKey(url, ua)
	valid := strings.Repeat("x", MaxDocumentBytes)
	if err := cacher.SetWithTTL(ctx, key, cachedDocumentValue(t, url, valid), time.Minute); err != nil {
		t.Fatal(err)
	}
	got := GetRemoteDocumentContext(ctx, url, "", 60, false)
	if got.Code != define.ERROR_CODE_NULL || got.Body != valid {
		t.Fatalf("boundary rejected: %d %d", got.Code, len(got.Body))
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	got = GetRemoteDocumentContext(canceled, url, "", 60, false)
	if got.Code == define.ERROR_CODE_NULL || got.Body != "" {
		t.Fatal("canceled caller got a successful cache document")
	}
}

func cachedDocumentValue(t *testing.T, url, body string) string {
	t.Helper()
	record := define.MixupRemoteBodySanitized(0, define.ERROR_STATUS_NULL, time.Now(), body)
	record.FinalURL = url
	record.HTTPStatus = 200
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
