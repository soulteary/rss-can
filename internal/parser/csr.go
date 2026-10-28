package parser

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/security"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
)

// Bound browser work, including connection, navigation and script evaluation.
var browserSlots = make(chan struct{}, 4)

func GetDataAndConfigByCSR(config define.JavaScriptConfig, container string, proxyAddr string) define.BodyParsed {
	return ParsePageByBrowser(config, container, proxyAddr, false)
}

func resolveBrowserURL(ctx context.Context, address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = define.DEFAULT_HEADLESS_SERVER
	}
	if _, err := strconv.Atoi(strings.TrimPrefix(address, ":")); err == nil {
		address = "127.0.0.1:" + strings.TrimPrefix(address, ":")
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	u, err := url.Parse(address)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "ws", "wss":
		if u.Path != "" && u.Path != "/" {
			return u.String(), nil
		}
		if u.Scheme == "ws" {
			u.Scheme = "http"
		} else {
			u.Scheme = "https"
		}
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported browser endpoint scheme")
	}
	u.Path = "/json/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("browser discovery returned %d", res.StatusCode)
	}
	var info struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.URL == "" {
		return "", fmt.Errorf("browser discovery returned no websocket URL")
	}
	websocketURL, err := url.Parse(info.URL)
	if err != nil {
		return "", err
	}
	if websocketURL.Scheme != "ws" && websocketURL.Scheme != "wss" {
		return "", fmt.Errorf("invalid browser websocket URL")
	}
	// Remote containers can advertise localhost. Keep the configured authority.
	websocketURL.Host = u.Host
	if u.Scheme == "https" {
		websocketURL.Scheme = "wss"
	}
	return websocketURL.String(), nil
}

// withBrowserPage owns an isolated browser context and its connection. It never
// closes the shared remote browser. Cleanup also runs after errors or timeouts.
func withBrowserPage(ctx context.Context, container, proxyAddr, targetURL, cookies string, publicOnly bool, action func(context.Context) error) error {
	select {
	case browserSlots <- struct{}{}:
		defer func() { <-browserSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}

	var allocCtx context.Context
	var allocCancel context.CancelFunc
	if define.CSR_DEBUG && !publicOnly {
		opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
		opts = append(opts, chromedp.VisibleWindow)
		if proxyAddr != "" {
			opts = append(opts, chromedp.ProxyServer(proxyAddr))
		}
		if define.CSR_IGNORE_CERT_ERRORS {
			opts = append(opts, chromedp.IgnoreCertErrors)
		}
		allocCtx, allocCancel = chromedp.NewExecAllocator(ctx, opts...)
	} else {
		address, err := resolveBrowserURL(ctx, container)
		if err != nil {
			return err
		}
		allocCtx, allocCancel = remote.NewAllocator(ctx, address, remote.NoModifyURL, remote.WithDialTimeout(time.Until(deadline(ctx))))
	}
	defer allocCancel()

	rootCtx, rootCancel := chromedp.NewContext(allocCtx)
	defer rootCancel()
	// Initialize the shared browser before creating an isolated BrowserContext.
	if err := chromedp.Do(rootCtx); err != nil {
		return err
	}
	pageCtx, pageCancel := chromedp.NewContext(rootCtx, chromedp.WithNewBrowserContext())
	defer pageCancel()
	if err := chromedp.Do(pageCtx); err != nil {
		return err
	}
	if define.CSR_IGNORE_CERT_ERRORS && !publicOnly {
		if _, err := cdp.Call(pageCtx, chromedp.FromContext(pageCtx).Target, security.SetIgnoreCertificateErrors, security.SetIgnoreCertificateErrorsParams{Ignore: true}); err != nil {
			return err
		}
	}
	if _, err := cdp.Call(pageCtx, chromedp.FromContext(pageCtx).Target, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: jssdk.CSR_NO_OP_ALERT}); err != nil {
		return err
	}
	if err := configureBrowserRequests(pageCtx, publicOnly); err != nil {
		return err
	}
	if err := setBrowserCookies(pageCtx, targetURL, cookies); err != nil {
		return err
	}
	return action(pageCtx)
}

func configureBrowserRequests(ctx context.Context, publicOnly bool) error {
	patterns := []*fetch.RequestPattern{
		{URLPattern: "*", ResourceType: cdpnetwork.ResourceTypeImage},
		{URLPattern: "*", ResourceType: cdpnetwork.ResourceTypeMedia},
		{URLPattern: "*", ResourceType: cdpnetwork.ResourceTypeFont},
	}
	if publicOnly {
		patterns = []*fetch.RequestPattern{{URLPattern: "*"}}
		if _, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, cdpnetwork.Enable, cdpnetwork.EnableParams{}); err != nil {
			return err
		}
		blocked := []*cdpnetwork.BlockPattern{
			{URLPattern: "ws://*:*/*", Block: true},
			{URLPattern: "wss://*:*/*", Block: true},
			{URLPattern: "file://*/*", Block: true},
			{URLPattern: "ftp://*:*/*", Block: true},
		}
		if _, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, cdpnetwork.SetBlockedURLs, cdpnetwork.SetBlockedURLsParams{URLPatterns: blocked}); err != nil {
			return err
		}
		if _, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, cdpnetwork.SetBypassServiceWorker, cdpnetwork.SetBypassServiceWorkerParams{Bypass: true}); err != nil {
			return err
		}
	}
	client := network.PublicHTTPClient(time.Until(deadline(ctx)))
	executor := chromedp.FromContext(ctx).Target
	events := cdp.Events(ctx, executor, fetch.RequestPaused)
	go func() {
		for paused, err := range events {
			if err != nil {
				return
			}
			paused := paused
			go func() {
				handlePausedRequest(ctx, &paused, publicOnly, client)
			}()
		}
	}()
	if _, err := cdp.Call(ctx, executor, fetch.Enable, fetch.EnableParams{Patterns: patterns}); err != nil {
		return err
	}
	return nil
}

func handlePausedRequest(ctx context.Context, paused *fetch.EventRequestPaused, publicOnly bool, client *http.Client) {
	executor := chromedp.FromContext(ctx).Target
	fail := func() {
		_, _ = cdp.Call(ctx, executor, fetch.FailRequest, fetch.FailRequestParams{RequestID: paused.RequestID, ErrorReason: cdpnetwork.ErrorReasonBlockedByClient})
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
	response, err := client.Do(req)
	if err != nil {
		fail()
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
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

func setBrowserCookies(ctx context.Context, targetURL, cookies string) error {
	if cookies == "" {
		return nil
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return err
	}
	response := &http.Response{Header: http.Header{"Set-Cookie": nil}}
	for _, raw := range strings.Split(cookies, ";") {
		response.Header.Set("Set-Cookie", strings.TrimSpace(raw))
		for _, cookie := range response.Cookies() {
			if _, err := cdp.Call(ctx, chromedp.FromContext(ctx).Target, cdpnetwork.SetCookie, cdpnetwork.SetCookieParams{Name: cookie.Name, Value: cookie.Value, URL: u.Scheme + "://" + u.Host + "/"}); err != nil {
				return err
			}
		}
	}
	return nil
}

func deadline(ctx context.Context) time.Time {
	d, ok := ctx.Deadline()
	if !ok {
		return time.Now().Add(5 * time.Second)
	}
	return d
}

func GetCSRInjectCode(file string) string {
	jsRule := fn.GetFileContent(file)
	if jsRule == nil {
		return ""
	}
	return jssdk.GenerateCSRInjectParser(jsRule)
}

func parseCSRItems(data string) define.BodyParsed {
	var items []define.InfoItem
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	return define.MixupBodyParsed(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, time.Now(), items)
}

func browserCacheKey(config define.JavaScriptConfig, useMixParser bool) string {
	// Separate browser JSON/HTML from SSR HTML, and isolate rules, cookies and
	// parser settings. Include the rule bytes so edits cannot reuse old results.
	encoded, _ := json.Marshal(config)
	rule := fn.GetFileContent(config.File)
	sum := sha256.Sum256(append(encoded, rule...))
	return fmt.Sprintf("browser:v2:%t:%x", useMixParser, sum)
}

func ParsePageByBrowser(config define.JavaScriptConfig, container string, proxyAddr string, useMixParser bool) define.BodyParsed {
	return ParsePageByBrowserContext(context.Background(), config, container, proxyAddr, useMixParser)
}

func ParsePageByBrowserContext(ctx context.Context, config define.JavaScriptConfig, container string, proxyAddr string, useMixParser bool) define.BodyParsed {
	if err := ctx.Err(); err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), nil)
	}
	key := browserCacheKey(config, useMixParser)
	decode := func(data string) define.BodyParsed {
		if useMixParser {
			return ParseDataAndConfigBySSRContext(ctx, config, define.RemoteBodySanitized{}, data)
		}
		return parseCSRItems(data)
	}
	if cacher.IsEnable() && !config.DisableCache {
		if cached, found, err := cacher.GetWithStatus(ctx, key); err == nil && found {
			return decode(cached)
		}
	}
	timeout := define.HEADLESS_EXCUTE_TIMEOUT
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, fn.ExpireBySecond(timeout))
	defer cancel()
	var data string
	err := withBrowserPage(ctx, container, proxyAddr, config.URL, config.Cookies, false, func(pageCtx context.Context) error {
		if err := chromedp.Do(pageCtx, chromedp.Navigate(config.URL), chromedp.WaitReady(chromedp.CSS(config.ListContainer))); err != nil {
			return err
		}
		if useMixParser {
			var err error
			data, err = chromedp.Run(pageCtx, chromedp.Evaluate[string]("("+jssdk.TPL_MIX_JS+")()"))
			return err
		}
		injectCode := GetCSRInjectCode(config.File)
		if injectCode == "" {
			return fmt.Errorf("cannot read rule %s", config.File)
		}
		var err error
		data, err = chromedp.Run(pageCtx, chromedp.Evaluate[string]("("+injectCode+")()"))
		return err
	})
	if err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), nil)
	}
	result := decode(data)
	if result.Code == define.ERROR_CODE_NULL && cacher.IsEnable() && !config.DisableCache {
		expire := config.Expire
		if expire <= 0 {
			expire = define.IN_MEMORY_EXPIRATION
		}
		if err := cacher.SetWithTTL(ctx, key, data, fn.ExpireBySecond(expire)); err != nil {
			logger.Instance.Warn("Unable to use cache")
		}
	}
	return result
}

func ProxyPageByBrowser(url string, container string, proxyAddr string, cookies string) (string, error) {
	if err := network.PublicURL(url); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT))
	defer cancel()
	var html string
	err := withBrowserPage(ctx, container, "", url, cookies, true, func(pageCtx context.Context) error {
		if err := chromedp.Do(pageCtx, chromedp.Navigate(url), chromedp.WaitReady(chromedp.CSS("body"))); err != nil {
			return err
		}
		var err error
		html, err = chromedp.Run(pageCtx, chromedp.Evaluate[string]("("+jssdk.TPL_MIX_JS+")()"))
		return err
	})
	return html, err
}
