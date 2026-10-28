package parser

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

func waitBrowserAdmissions(t *testing.T, queue *browserAdmission, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for len(queue.admitted) != count {
		if time.Now().After(deadline) {
			t.Fatalf("admitted=%d want=%d", len(queue.admitted), count)
		}
		runtime.Gosched()
	}
}

func TestBrowserAdmissionBoundsAndCancellation(t *testing.T) {
	queue := newBrowserAdmission(2, 3)
	first, err := queue.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	second, err := queue.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	results := make(chan error, 3)
	for range 3 {
		go func() {
			release, err := queue.acquire(ctx)
			if release != nil {
				release()
			}
			results <- err
		}()
	}
	waitBrowserAdmissions(t, queue, 5)
	if release, err := queue.acquire(t.Context()); release != nil || !errors.Is(err, ErrBrowserOverloaded) {
		t.Fatalf("full queue: release=%v err=%v", release != nil, err)
	}
	if len(queue.active) != 2 {
		t.Fatalf("active=%d", len(queue.active))
	}
	cancel()
	for range 3 {
		if err := <-results; !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter=%v", err)
		}
	}
	waitBrowserAdmissions(t, queue, 2)
	first()
	first()
	second()
	second()
	if len(queue.active) != 0 || len(queue.admitted) != 0 {
		t.Fatal("slots leaked after idempotent release")
	}
	canceled, stop := context.WithCancel(t.Context())
	stop()
	if release, err := queue.acquire(canceled); release != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled=%v", err)
	}
}

func TestBrowserAdmissionWakesWaiterAndExpires(t *testing.T) {
	queue := newBrowserAdmission(1, 1)
	release, _ := queue.acquire(t.Context())
	result := make(chan error, 1)
	go func() {
		done, err := queue.acquire(t.Context())
		if done != nil {
			done()
		}
		result <- err
	}()
	waitBrowserAdmissions(t, queue, 2)
	release()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if len(queue.admitted) != 0 || len(queue.active) != 0 {
		t.Fatal("waiter leaked slots")
	}
	release, _ = queue.acquire(t.Context())
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	if done, err := queue.acquire(ctx); done != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout=%v", err)
	}
	release()
	if len(queue.admitted) != 0 {
		t.Fatal("expired waiter leaked admission")
	}
}

// Full process admission is rejected before discovery, for both feed modes and
// Inspector. Existing work and queued waiters do not lose their slots.
func TestBrowserOverloadPreservesFeedCodeAndMetrics(t *testing.T) {
	for range cap(browserQueue.admitted) {
		browserQueue.admitted <- struct{}{}
	}
	defer func() {
		for range cap(browserQueue.admitted) {
			<-browserQueue.admitted
		}
	}()
	queuedBefore := telemetry.Default.Value("browser_queued")
	activeBefore := telemetry.Default.Value("browser_active")
	for _, mix := range []bool{false, true} {
		config := define.JavaScriptConfig{URL: "https://example.org", ListContainer: "article", Title: "h2", DisableCache: true, Timeout: 1}
		result := ParsePageByBrowserContext(t.Context(), config, "http://127.0.0.1:1", "", mix)
		if result.Code != define.ERROR_CODE_OVERLOADED || !strings.Contains(result.Status, "browser queue is full") {
			t.Fatalf("mix=%v result=%+v", mix, result)
		}
	}
	if _, err := ProxyPageByBrowserWithTimeout(t.Context(), "https://example.org", "http://127.0.0.1:1", "", "", time.Second); !errors.Is(err, ErrBrowserOverloaded) {
		t.Fatalf("inspector overload=%v", err)
	}
	if telemetry.Default.Value("browser_queued") != queuedBefore || telemetry.Default.Value("browser_active") != activeBefore {
		t.Fatal("overload changed queue/active gauges")
	}
	if len(browserQueue.active) != 0 {
		t.Fatal("overload allocated active slot")
	}
}
