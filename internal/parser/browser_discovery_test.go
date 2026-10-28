package parser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type discoveryTransport func(*http.Request) (*http.Response, error)

func (f discoveryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type discoveryBody struct {
	io.Reader
	reads, closes int
}

func (b *discoveryBody) Read(p []byte) (int, error) {
	n, e := b.Reader.Read(p)
	b.reads += n
	return n, e
}
func (b *discoveryBody) Close() error { b.closes++; return nil }

func TestBrowserDiscoveryResponseBudget(t *testing.T) {
	valid := `{"webSocketDebuggerUrl":"ws://localhost:9222/devtools/browser/test"}`
	for _, tc := range []struct {
		name          string
		size          int
		contentLength int64
		wantError     bool
		maxRead       int
	}{
		{"declared oversize", maxBrowserDiscoveryBytes + 1, maxBrowserDiscoveryBytes + 1, true, 0},
		{"unknown oversize", maxBrowserDiscoveryBytes + 1, -1, true, maxBrowserDiscoveryBytes + 1},
		{"lying length", maxBrowserDiscoveryBytes + 1, 1, true, maxBrowserDiscoveryBytes + 1},
		{"exact boundary", maxBrowserDiscoveryBytes, -1, false, maxBrowserDiscoveryBytes},
		{"ordinary response", len(valid), int64(len(valid)), false, len(valid)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &discoveryBody{Reader: strings.NewReader(valid + strings.Repeat(" ", tc.size-len(valid)))}
			previous := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: discoveryTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: body, ContentLength: tc.contentLength}, nil
			})}
			defer func() { http.DefaultClient = previous }()
			got, err := resolveBrowserURL(t.Context(), "http://remote.example:19239")
			if (err != nil) != tc.wantError {
				t.Fatalf("url=%s error=%v", got, err)
			}
			if !tc.wantError && got != "ws://remote.example:19239/devtools/browser/test" {
				t.Fatalf("authority lost: %s", got)
			}
			if body.reads > tc.maxRead || body.closes != 1 {
				t.Fatalf("read=%d max=%d close=%d", body.reads, tc.maxRead, body.closes)
			}
		})
	}
}

func TestBrowserDiscoveryEndpointURLBudget(t *testing.T) {
	for _, scheme := range []string{"ws://", "wss://", "http://"} {
		address := scheme + "remote.example/" + strings.Repeat("x", maxBrowserEndpointURLBytes)
		if _, err := resolveBrowserURL(t.Context(), address); err == nil {
			t.Fatalf("accepted oversized %s endpoint", scheme)
		}
	}
	if _, err := resolveBrowserURL(t.Context(), "ws://remote.example/"+strings.Repeat("界", 2700)); err == nil {
		t.Fatal("accepted oversized percent-encoded endpoint")
	}
	valid := "ws://remote.example/" + strings.Repeat("x", maxBrowserEndpointURLBytes-len("ws://remote.example/"))
	if got, err := resolveBrowserURL(t.Context(), valid); err != nil || got != valid {
		t.Fatalf("exact URL boundary: %s %v", got, err)
	}
	for _, size := range []int{maxBrowserEndpointURLBytes, maxBrowserEndpointURLBytes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			advertised := "ws://localhost/" + strings.Repeat("x", size-len("ws://localhost/"))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"webSocketDebuggerUrl":%q}`, advertised)
			}))
			defer server.Close()
			got, err := resolveBrowserURL(t.Context(), server.URL)
			// The advertised URL itself is checked, and replacing localhost with the
			// configured authority must also stay within the URL budget.
			if err == nil || got != "" {
				t.Fatalf("accepted expanded/oversized URL: len=%d %v", len(got), err)
			}
		})
	}
}

func TestBrowserDiscoveryHTTPSAndValidation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"webSocketDebuggerUrl":"ws://localhost:9222/devtools/browser/test"}`)
	}))
	defer server.Close()
	previous := http.DefaultClient
	http.DefaultClient = server.Client()
	defer func() { http.DefaultClient = previous }()
	got, err := resolveBrowserURL(t.Context(), server.URL)
	if want := strings.Replace(server.URL, "https://", "wss://", 1) + "/devtools/browser/test"; err != nil || got != want {
		t.Fatalf("got %s %v want %s", got, err, want)
	}
	for _, body := range []string{`{}`, `{"webSocketDebuggerUrl":"http://wrong.example/path"}`, `{"webSocketDebuggerUrl":"ws://localhost/path"} trailing`} {
		t.Run(body, func(t *testing.T) {
			responseBody := &discoveryBody{Reader: strings.NewReader(body)}
			http.DefaultClient = &http.Client{Transport: discoveryTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: responseBody, ContentLength: -1}, nil
			})}
			if _, err := resolveBrowserURL(t.Context(), "http://remote.example"); err == nil {
				t.Fatal("accepted invalid discovery payload")
			}
			if responseBody.closes != 1 {
				t.Fatalf("close=%d", responseBody.closes)
			}
		})
	}
}

func TestBrowserDiscoveryCancellationDuringBody(t *testing.T) {
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"padding":"`)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := resolveBrowserURL(ctx, server.URL); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("body cancellation: %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("discovery response did not close on cancellation")
	}
	ctx, cancel = context.WithCancel(t.Context())
	cancel()
	if _, err := resolveBrowserURL(ctx, "ws://localhost/direct"); !errors.Is(err, context.Canceled) {
		t.Fatalf("direct endpoint ignored cancellation: %v", err)
	}
}

func TestBrowserDiscoveryChunkedResponseBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("discovery must keep the wire byte budget")
		}
		fmt.Fprint(w, `{"webSocketDebuggerUrl":"ws://localhost/devtools/browser/test"}`)
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, strings.Repeat(" ", maxBrowserDiscoveryBytes))
	}))
	defer server.Close()
	if _, err := resolveBrowserURL(t.Context(), server.URL); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("chunked discovery response bypassed budget: %v", err)
	}
}

func TestBrowserDiscoveryRejectsCompressedResponses(t *testing.T) {
	for _, encoding := range []string{"gzip", "br"} {
		t.Run(encoding, func(t *testing.T) {
			body := &discoveryBody{Reader: strings.NewReader("compressed bytes")}
			previous := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: discoveryTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: body, ContentLength: -1, Header: http.Header{"Content-Encoding": []string{encoding}}}, nil
			})}
			defer func() { http.DefaultClient = previous }()
			if _, err := resolveBrowserURL(t.Context(), "http://remote.example"); err == nil || !strings.Contains(err.Error(), "identity encoding") {
				t.Fatalf("accepted compressed discovery: %v", err)
			}
			if body.reads != 0 || body.closes != 1 {
				t.Fatalf("read=%d close=%d", body.reads, body.closes)
			}
		})
	}
}
