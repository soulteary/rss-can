package parser

import (
	"context"
	"errors"
	"fmt"
	"github.com/chromedp/chromedp"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
)

func TestSSRRejectsPageItemAndPaginationLimits(t *testing.T) {
	base := define.JavaScriptConfig{URL: "https://example.org", ListContainer: "article", Title: "h2"}
	cases := []struct {
		name   string
		config define.JavaScriptConfig
		html   string
	}{
		{"page", base, strings.Repeat("x", limits.MaxPageBytes+1)},
		{"items", base, strings.Repeat("<article><h2>title</h2></article>", limits.MaxItems+1)},
	}
	pager := base
	pager.PagerLimit = limits.MaxPages + 1
	cases = append(cases, struct {
		name   string
		config define.JavaScriptConfig
		html   string
	}{"pages", pager, "<article><h2>title</h2></article>"})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseDataAndConfigBySSRContext(t.Context(), tc.config, define.RemoteBodySanitized{}, tc.html)
			if got.Code == 0 || len(got.Body) != 0 {
				t.Fatalf("partial/accepted result: %d %d", got.Code, len(got.Body))
			}
		})
	}
	tooMany := "[" + strings.TrimSuffix(strings.Repeat(`{"title":"item"},`, limits.MaxItems+1), ",") + "]"
	if got := parseCSRItems(tooMany); got.Code == 0 || len(got.Body) != 0 {
		t.Fatal("CSR item limit ignored")
	}
}

func TestBrowserDataSharesWorkWithIndependentCancellation(t *testing.T) {
	memoryCacheForTest(t)
	key := "browser-share:" + t.Name()
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	fetch := func(ctx context.Context) (string, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		select {
		case <-release:
			return "page", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	firstCtx, cancel := context.WithCancel(t.Context())
	first := make(chan error, 1)
	go func() { _, err := browserData(firstCtx, key, true, time.Second, 1, fetch); first <- err }()
	<-entered
	second := make(chan error, 1)
	go func() {
		value, err := browserData(t.Context(), key, true, time.Second, 1, fetch)
		if value != "page" && err == nil {
			err = fmt.Errorf("unexpected %q", value)
		}
		second <- err
	}()
	// Allow the second caller to join the existing flight before its leader leaves.
	time.Sleep(30 * time.Millisecond)
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader cancel: %v", err)
	}
	close(release)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("duplicate browser work: %d", calls.Load())
	}
	if value, err := browserData(t.Context(), key, true, time.Second, 1, fetch); err != nil || value != "page" || calls.Load() != 1 {
		t.Fatal("shared result not cached")
	}
}

func TestBrowserDataLastWaiterCancelsAndDisableCacheDoesNotShare(t *testing.T) {
	memoryCacheForTest(t)
	ctx, cancel := context.WithCancel(t.Context())
	entered, canceled := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := browserData(ctx, "browser-last:"+t.Name(), true, time.Second, 1, func(work context.Context) (string, error) {
			close(entered)
			<-work.Done()
			close(canceled)
			return "", work.Err()
		})
		done <- err
	}()
	<-entered
	cancel()
	<-done
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("last waiter left browser work alive")
	}
	enteredMany := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := browserData(t.Context(), "no-share", false, time.Second, 1, func(context.Context) (string, error) { enteredMany <- struct{}{}; <-release; return "page", nil })
			results <- err
		}()
	}
	for range 2 {
		select {
		case <-enteredMany:
		case <-time.After(time.Second):
			t.Fatal("DisableCache unexpectedly shared")
		}
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if data, err := browserData(t.Context(), "oversized", false, time.Second, 1, func(context.Context) (string, error) { return strings.Repeat("x", limits.MaxPageBytes+1), nil }); err == nil || data != "" {
		t.Fatal("oversized browser result accepted")
	}
}

func TestLiveBrowserRejectsOversizeBeforeTransfer(t *testing.T) {
	address := os.Getenv("RSS_TEST_BROWSER_URL")
	if address == "" {
		if os.Getenv("RSS_TEST_BROWSER_REQUIRED") == "1" {
			t.Fatal("browser required")
		}
		t.Skip("requires isolated Chrome")
	}
	for _, expression := range []string{fmt.Sprintf("'x'.repeat(%d)", limits.MaxPageBytes+1), fmt.Sprintf("'你'.repeat(%d)", limits.MaxPageBytes/2)} {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		err := withBrowserPage(ctx, address, "", "about:blank", "", false, func(pageCtx context.Context) error {
			_, err := chromedp.Run(pageCtx, chromedp.Evaluate[string](boundedBrowserExpression(expression)))
			return err
		})
		cancel()
		if err == nil || !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("browser allowed oversized result: %v", err)
		}
	}
}
