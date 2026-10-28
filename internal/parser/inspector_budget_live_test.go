package parser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/fetch"
	cdpnetwork "github.com/chromedp/cdproto/network"
)

type inspectorTransport func(*http.Request) (*http.Response, error)

func (f inspectorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestLiveInspectorInterceptionKeepsPolicyAndBoundedBodies(t *testing.T) {
	endpoint := os.Getenv("RSS_TEST_BROWSER_URL")
	if endpoint == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("browser endpoint required")
		}
		t.Skip("requires isolated Chrome")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := withBrowserPage(ctx, endpoint, "", "about:blank", "", false, func(pageCtx context.Context) error {
		paused := &fetch.EventRequestPaused{RequestID: "inspector-budget-test", ResourceType: cdpnetwork.ResourceTypeScript, Request: &cdpnetwork.Request{URL: "https://1.1.1.1/script.js", Method: "POST", Headers: cdpnetwork.Headers{"X-Test": "kept", "Host": "ignored", "Numeric": 12, "Accept-Encoding": "br"}, PostDataEntries: []*cdpnetwork.PostDataEntry{{Bytes: []byte("posted")}}}}
		response, body := inspectorResponse(strings.NewReader("small script"), -1, "")
		response.Status = "200 OK"
		response.Header.Set("Content-Type", "application/javascript")
		calls := 0
		client := &http.Client{Transport: inspectorTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("Accept-Encoding") != "identity" || r.Header.Get("X-Test") != "kept" || r.Header.Get("Host") != "" {
				t.Errorf("incorrect forwarded headers: %v", r.Header)
			}
			posted, _ := io.ReadAll(r.Body)
			if string(posted) != "posted" {
				t.Errorf("postdata %q", posted)
			}
			return response, nil
		})}
		handlePausedRequest(pageCtx, paused, true, client)
		if calls != 1 || body.reads.Load() != int64(len("small script")) || body.closes.Load() != 1 {
			t.Errorf("response not bounded/closed: calls=%d bytes=%d closes=%d", calls, body.reads.Load(), body.closes.Load())
		}
		blocked := *paused
		blocked.ResourceType = cdpnetwork.ResourceTypeImage
		handlePausedRequest(pageCtx, &blocked, true, client)
		private := *paused
		private.Request = &cdpnetwork.Request{URL: "http://127.0.0.1/private", Method: "GET"}
		handlePausedRequest(pageCtx, &private, true, client)
		invalid := *paused
		invalid.Request = &cdpnetwork.Request{URL: "https://1.1.1.1/script.js", Method: "invalid method"}
		handlePausedRequest(pageCtx, &invalid, true, client)
		handlePausedRequest(pageCtx, paused, false, client)
		if calls != 1 {
			t.Errorf("blocked/private/continued request was fetched: %d", calls)
		}
		failed := &http.Client{Transport: inspectorTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("failed request") })}
		handlePausedRequest(pageCtx, paused, true, failed)
		oversized, oversizedBody := inspectorResponse(strings.NewReader(strings.Repeat("x", InspectorResourceBytes+100)), InspectorResourceBytes+100, "")
		handlePausedRequest(pageCtx, paused, true, &http.Client{Transport: inspectorTransport(func(*http.Request) (*http.Response, error) { return oversized, nil })})
		if oversizedBody.reads.Load() != 0 || oversizedBody.closes.Load() != 1 {
			t.Error("oversized declared resource was read or not closed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the actual public-page controller setup/drain without issuing a
	// remote navigation. Policy validation and transport bodies above stay local.
	err = withBrowserPage(ctx, endpoint, "", "about:blank", "", true, func(pageCtx context.Context) error {
		canceled, cancel := context.WithCancel(pageCtx)
		cancel()
		if err := configureBrowserRequests(canceled, false); !errors.Is(err, context.Canceled) {
			t.Errorf("canceled setup: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
