package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func waitForAdmissions(t *testing.T, b *fetchBudget, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for len(b.admitted) != want {
		if time.Now().After(deadline) {
			t.Fatalf("admitted=%d, want %d", len(b.admitted), want)
		}
		runtime.Gosched()
	}
}

func TestFetchBudgetQueueCancellationAndOverload(t *testing.T) {
	b := newFetchBudget(1, 1)
	release, err := b.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		r, err := b.acquire(ctx)
		if r != nil {
			r()
		}
		done <- err
	}()
	waitForAdmissions(t, b, 2)
	if _, err := b.acquire(context.Background()); !errors.Is(err, ErrFetchOverloaded) {
		t.Fatalf("saturated budget: %v", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued cancellation: %v", err)
	}
	waitForAdmissions(t, b, 1)
	release()
	release() // Body EOF followed by Close must release only once.
	if len(b.active) != 0 || len(b.admitted) != 0 {
		t.Fatal("slots leaked")
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err := b.acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestFetchBudgetQueuedRequestProgresses(t *testing.T) {
	b := newFetchBudget(1, 1)
	release, _ := b.acquire(context.Background())
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		r, err := b.acquire(ctx)
		if r != nil {
			r()
		}
		done <- err
	}()
	waitForAdmissions(t, b, 2)
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(b.active) != 0 || len(b.admitted) != 0 {
		t.Fatal("queued work leaked slots")
	}
}

func useTestFetchBudget(t *testing.T, concurrency, queued int) *fetchBudget {
	t.Helper()
	previous := upstreamBudget
	b := newFetchBudget(concurrency, queued)
	upstreamBudget = b
	t.Cleanup(func() { upstreamBudget = previous })
	return b
}

func TestHTTPBudgetRetainedUntilBodyReadOrClose(t *testing.T) {
	b := useTestFetchBudget(t, 1, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "body")
	}))
	defer server.Close()
	for _, consume := range []bool{false, true} {
		code, status, response := HttpGetContext(context.Background(), server.URL, "")
		if code != 0 {
			t.Fatal(status)
		}
		if len(b.active) != 1 {
			t.Fatal("slot released at headers")
		}
		if code, status, _ := HttpGetContext(context.Background(), server.URL, ""); code != define.ERROR_CODE_OVERLOADED || !strings.Contains(status, ErrFetchOverloaded.Error()) {
			t.Fatalf("unbounded second request: %d %s", code, status)
		}
		if consume {
			if _, err := io.ReadAll(response.Body); err != nil {
				t.Fatal(err)
			}
			if len(b.active) != 0 {
				t.Fatal("EOF did not release slot")
			}
		}
		response.Body.Close()
		response.Body.Close()
		if len(b.active) != 0 || len(b.admitted) != 0 {
			t.Fatal("closed body leaked slot")
		}
	}
	if code, _, _ := HttpGetContext(context.Background(), "http://127.0.0.1:0", ""); code == 0 {
		t.Fatal("failed connection accepted")
	}
	if len(b.active) != 0 || len(b.admitted) != 0 {
		t.Fatal("failed connection leaked slot")
	}
}

func TestDocumentCacheHitBypassesBudgetAndDisabledCacheIsLimited(t *testing.T) {
	logger.Initialize()
	b := useTestFetchBudget(t, 1, 0)
	redis, memory := define.REDIS, define.IN_MEMORY_CACHE
	define.REDIS, define.IN_MEMORY_CACHE = false, true
	defer func() { define.REDIS, define.IN_MEMORY_CACHE = redis, memory }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "cached")
	}))
	defer server.Close()
	if result := GetRemoteDocument(server.URL, "utf-8", 30, false); result.Code != 0 {
		t.Fatal(result.Status)
	}
	release, _ := b.acquire(context.Background())
	defer release()
	if result := GetRemoteDocument(server.URL, "utf-8", 30, false); result.Code != 0 || result.Body != "cached" {
		t.Fatalf("cached document used fetch budget: %#v", result)
	}
	if result := GetRemoteDocument(server.URL, "utf-8", 30, true); result.Code != define.ERROR_CODE_OVERLOADED || !strings.Contains(result.Status, ErrFetchOverloaded.Error()) {
		t.Fatalf("DisableCache bypassed fetch budget: %#v", result)
	}
}
