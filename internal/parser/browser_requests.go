package parser

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/workscope"
)

func configureBrowserRequests(ctx context.Context, publicOnly bool) error {
	_, err := configureBrowserRequestsWithBudget(ctx, publicOnly)
	return err
}

func configureBrowserRequestsWithBudget(ctx context.Context, publicOnly bool) (*inspectorInterception, error) {
	patterns := []*fetch.RequestPattern{
		{URLPattern: "*", ResourceType: cdpnetwork.ResourceTypeImage},
		{URLPattern: "*", ResourceType: cdpnetwork.ResourceTypeMedia},
		{URLPattern: "*", ResourceType: cdpnetwork.ResourceTypeFont},
	}
	if publicOnly {
		patterns = []*fetch.RequestPattern{{URLPattern: "*"}}
		if _, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, cdpnetwork.Enable, cdpnetwork.EnableParams{}); err != nil {
			return nil, err
		}
		blocked := []*cdpnetwork.BlockPattern{
			{URLPattern: "ws://*:*/*", Block: true},
			{URLPattern: "wss://*:*/*", Block: true},
			{URLPattern: "file://*/*", Block: true},
			{URLPattern: "ftp://*:*/*", Block: true},
		}
		if _, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, cdpnetwork.SetBlockedURLs, cdpnetwork.SetBlockedURLsParams{URLPatterns: blocked}); err != nil {
			return nil, err
		}
		if _, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, cdpnetwork.SetBypassServiceWorker, cdpnetwork.SetBypassServiceWorkerParams{Bypass: true}); err != nil {
			return nil, err
		}
	}
	client := network.PublicHTTPClient(time.Until(deadline(ctx)))
	executor := chromedp.FromContext(ctx).Target
	budget := newInspectorBudget()
	work, cancel := context.WithCancel(ctx)
	events := cdp.Events(work, executor, fetch.RequestPaused)
	dispatcher := newInspectorDispatcher(work, func(taskCtx context.Context, paused *fetch.EventRequestPaused) {
		handlePausedRequestWithBudget(taskCtx, paused, publicOnly, client, budget)
	}, func(paused *fetch.EventRequestPaused) {
		if publicOnly {
			budget.record(errInspectorQueueBudget)
		}
		failPausedRequest(work, paused)
	})
	workscope.Go(ctx, func() {
		defer cancel()
		defer client.CloseIdleConnections()
		for paused, err := range events {
			if err != nil {
				return
			}
			paused := paused
			dispatcher.submit(&paused)
		}
	})
	if _, err := cdp.Call(ctx, executor, fetch.Enable, fetch.EnableParams{Patterns: patterns}); err != nil {
		cancel()
		return nil, err
	}
	return &inspectorInterception{inspectorBudget: budget, dispatcher: dispatcher}, nil
}

func failPausedRequest(ctx context.Context, paused *fetch.EventRequestPaused) {
	_, _ = cdp.Call(ctx, chromedp.FromContext(ctx).Target, fetch.FailRequest, fetch.FailRequestParams{RequestID: paused.RequestID, ErrorReason: cdpnetwork.ErrorReasonBlockedByClient})
}

// Compatibility helper: production interceptions share one budget per page.
func handlePausedRequest(ctx context.Context, paused *fetch.EventRequestPaused, publicOnly bool, client *http.Client) {
	handlePausedRequestWithBudget(ctx, paused, publicOnly, client, newInspectorBudget())
}
func handlePausedRequestWithBudget(ctx context.Context, paused *fetch.EventRequestPaused, publicOnly bool, client *http.Client, budget *inspectorBudget) {
	executor := chromedp.FromContext(ctx).Target
	fail := func() { failPausedRequest(ctx, paused) }
	if ctx.Err() != nil {
		fail()
		return
	}
	if paused.ResourceType == cdpnetwork.ResourceTypeImage || paused.ResourceType == cdpnetwork.ResourceTypeMedia || paused.ResourceType == cdpnetwork.ResourceTypeFont {
		fail()
		return
	}
	if !publicOnly {
		_, _ = cdp.Call(ctx, executor, fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: paused.RequestID})
		return
	}
	if err := network.PublicURL(paused.Request.URL); err != nil {
		fail()
		return
	}
	postData := make([][]byte, 0, len(paused.Request.PostDataEntries))
	for _, entry := range paused.Request.PostDataEntries {
		postData = append(postData, entry.Bytes)
	}
	req, err := http.NewRequestWithContext(ctx, paused.Request.Method, paused.Request.URL, bytes.NewReader(bytes.Join(postData, nil)))
	if err != nil {
		fail()
		return
	}
	for name, value := range paused.Request.Headers {
		if text, ok := value.(string); ok && !strings.EqualFold(name, "host") {
			req.Header.Set(name, text)
		}
	}
	// Request identity encoding so the Transport cannot decompress before the
	// wire limit is applied; bounded gzip decoding is handled below if needed.
	req.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(req)
	if err != nil {
		fail()
		return
	}
	body, err := readInspectorResponse(ctx, response, budget)
	if err != nil {
		fail()
		return
	}
	headers := make([]*fetch.HeaderEntry, 0, len(response.Header))
	for name, values := range response.Header {
		for _, value := range values {
			headers = append(headers, &fetch.HeaderEntry{Name: name, Value: value})
		}
	}
	_, _ = cdp.Call(ctx, executor, fetch.FulfillRequest, fetch.FulfillRequestParams{
		RequestID:       paused.RequestID,
		ResponseCode:    int64(response.StatusCode),
		ResponseHeaders: headers,
		Body:            body,
		ResponsePhrase:  strings.TrimSpace(strings.TrimPrefix(response.Status, strconv.Itoa(response.StatusCode))),
	})
}
