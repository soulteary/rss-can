package network

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestTruncatedUpstreamResponseIsAnError(t *testing.T) {
	logger.Initialize()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.Write([]byte("partial"))
	}))
	defer server.Close()
	got := GetRemoteDocument(server.URL, "", 1, true)
	if got.Code != define.ERROR_CODE_NETWORK || got.Body != "" {
		t.Fatalf("truncated body accepted: %#v", got)
	}
}

func TestInspectorDestinationPolicy(t *testing.T) {
	for _, raw := range []string{"", "file:///etc/passwd", "ftp://example.org", "http://localhost", "http://127.0.0.1", "http://[::1]", "http://169.254.169.254", "http://10.0.0.1", "http://[::ffff:127.0.0.1]", "https://user:password@example.org"} {
		if err := PublicURL(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, ip := range []string{"127.0.0.1", "169.254.169.254", "10.0.0.1", "172.16.0.1", "192.168.0.1", "::1", "fc00::1", "fe80::1", "100.100.100.200", "0.0.0.0", "::ffff:127.0.0.1", "64:ff9b::7f00:1"} {
		if isPublicIP(netip.MustParseAddr(ip)) {
			t.Errorf("accepted private address %s", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !isPublicIP(netip.MustParseAddr(ip)) {
			t.Errorf("blocked public address %s", ip)
		}
	}
	if err := PublicURL("https://example.org/feed"); err != nil {
		t.Fatal(err)
	}
}

func TestInspectorTransportBlocksDNSAndRedirectDestinations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, destination := range []string{"127.0.0.1:80", "localhost:80", "[::1]:80", "169.254.169.254:80"} {
		conn, err := publicDialContext(ctx, "tcp", destination)
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Errorf("dialed %s", destination)
		}
	}
	client := PublicHTTPClient(time.Second)
	if err := client.CheckRedirect(&http.Request{}, nil); err != http.ErrUseLastResponse {
		t.Fatal("redirects must be re-intercepted")
	}
	// A redirect/subresource request to a private IP uses the same pinned transport.
	if _, err := client.Get("http://127.0.0.1:80"); err == nil {
		t.Fatal("private redirect destination allowed")
	}
}

func TestDocumentBodyHonorsContextDeadline(t *testing.T) {
	logger.Initialize()
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	got := GetRemoteDocumentContext(ctx, server.URL, "utf-8", 1, true)
	if got.Code == define.ERROR_CODE_NULL || got.Body != "" || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("deadline lost: %#v", got)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("body request not canceled")
	}
}

func TestOversizedDocumentRejected(t *testing.T) {
	logger.Initialize()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", MaxDocumentBytes+1))
	}))
	defer server.Close()
	got := GetRemoteDocument(server.URL, "utf-8", 1, true)
	if got.Code == define.ERROR_CODE_NULL || got.Body != "" || !strings.Contains(got.Status, "10 MiB") {
		t.Fatalf("oversized document accepted: code=%v status=%q size=%d", got.Code, got.Status, len(got.Body))
	}
}
