package parser

import (
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
	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/security"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/coalesce"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/rulefile"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

// Bound browser work, including connection, navigation and script evaluation.
var browserSlots = make(chan struct{}, 4)
var browserFlights coalesce.Group[string]

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
func withBrowserPage(ctx context.Context, container, proxyAddr, targetURL, cookies string, publicOnly bool, action func(context.Context) error) (err error) {
	telemetry.Default.Add("browser_requests_total", 1)
	began := time.Now()
	telemetry.Default.Add("browser_queued", 1)
	defer func() {
		if err != nil {
			telemetry.Default.Add("browser_failures_total", 1)
		}
	}()
	select {
	case browserSlots <- struct{}{}:
		telemetry.Default.Add("browser_queued", -1)
		telemetry.Default.Observe("browser_queue_seconds", time.Since(began))
		telemetry.Default.Add("browser_active", 1)
		began = time.Now()
		defer func() {
			<-browserSlots
			telemetry.Default.Add("browser_active", -1)
			telemetry.Default.Observe("browser_duration_seconds", time.Since(began))
		}()
	case <-ctx.Done():
		telemetry.Default.Add("browser_queued", -1)
		telemetry.Default.Observe("browser_queue_seconds", time.Since(began))
		return ctx.Err()
	}

	var allocCtx context.Context
	var allocCancel context.CancelFunc
	options := browserOptions(ctx)
	if options.Debug && !publicOnly {
		opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
		opts = append(opts, chromedp.VisibleWindow)
		if proxyAddr != "" {
			opts = append(opts, chromedp.ProxyServer(proxyAddr))
		}
		if options.IgnoreCertificateErrors {
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
	pageCtx, pageCancel := chromedp.NewContext(rootCtx, chromedp.WithNewBrowserContext(func(params *target.CreateBrowserContextParams) {
		if !publicOnly {
			params.ProxyServer = proxyAddr
		}
	}))
	defer pageCancel()
	if err := chromedp.Do(pageCtx); err != nil {
		return err
	}
	if options.IgnoreCertificateErrors && !publicOnly {
		if _, err := cdp.Call(pageCtx, chromedp.FromContext(pageCtx).Target, security.SetIgnoreCertificateErrors, security.SetIgnoreCertificateErrorsParams{Ignore: true}); err != nil {
			return err
		}
	}
	if _, err := cdp.Call(pageCtx, chromedp.FromContext(pageCtx).Target, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: jssdk.CSR_NO_OP_ALERT}); err != nil {
		return err
	}
	interception, err := configureBrowserRequestsWithBudget(pageCtx, publicOnly)
	if err != nil {
		return err
	}
	if err := setBrowserCookies(pageCtx, targetURL, cookies); err != nil {
		return err
	}
	return interception.result(action(pageCtx), publicOnly)
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

// GetCSRInjectCode is a compatibility helper for callers loading a standalone
// file. Feed generation uses csrInjectCode with its captured configuration source.
func GetCSRInjectCode(file string) string {
	jsRule, err := rulefile.Read(context.Background(), file)
	if err != nil {
		return ""
	}
	return jssdk.GenerateCSRInjectParser(jsRule)
}

func parseCSRItems(data string) define.BodyParsed {
	failure := func(message string) define.BodyParsed {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, message, time.Now(), nil)
	}
	if len(data) > limits.MaxPageBytes {
		return failure("browser result exceeds size limit")
	}
	decoder := json.NewDecoder(strings.NewReader(data))
	start, err := decoder.Token()
	if err != nil {
		return failure(err.Error())
	}
	// Historical JSON null represented an empty result.
	if start == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return failure("invalid trailing browser JSON")
		}
		return define.MixupBodyParsed(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, time.Now(), nil)
	}
	if start != json.Delim('[') {
		return failure("browser result must be an array")
	}
	var items []define.InfoItem
	for decoder.More() {
		if len(items) >= limits.MaxItems {
			return failure("browser result exceeds item limit")
		}
		var item define.InfoItem
		if err := decoder.Decode(&item); err != nil {
			return failure(err.Error())
		}
		items = append(items, item)
	}
	if _, err := decoder.Token(); err != nil {
		return failure(err.Error())
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return failure("invalid trailing browser JSON")
	}
	return define.MixupBodyParsed(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, time.Now(), items)
}

func browserCacheKey(config define.JavaScriptConfig, useMixParser bool) string {
	return browserCacheKeyContext(context.Background(), config, useMixParser)
}

func browserCacheKeyContext(ctx context.Context, config define.JavaScriptConfig, useMixParser bool) string {
	// Separate browser JSON/HTML from SSR HTML, and isolate rules, cookies and
	// parser settings. Include the rule bytes so edits cannot reuse old results.
	// Include the effective global proxy when a rule does not override it.
	config.Proxy = network.OptionsForConfigContext(ctx, config).Proxy
	if normalized, err := network.NormalizeProxy(config.Proxy); err == nil {
		config.Proxy = normalized
	}
	encoded, _ := json.Marshal(config)
	ruleHash := config.RuleSource.Hash()
	sum := sha256.Sum256(append(encoded, ruleHash[:]...))
	return fmt.Sprintf("browser:v6:%t:%x", useMixParser, sum)
}

// Direct parser callers may supply only File. Capture it once, before cache
// lookup and browser work; production configurations already carry their source.
func captureBrowserRule(config define.JavaScriptConfig) (define.JavaScriptConfig, error) {
	return captureBrowserRuleContext(context.Background(), config)
}
func captureBrowserRuleContext(ctx context.Context, config define.JavaScriptConfig) (define.JavaScriptConfig, error) {
	if err := ctx.Err(); err != nil {
		return config, err
	}
	if !config.RuleSource.Present() && config.File != "" {
		body, err := rulefile.Read(ctx, config.File)
		if err != nil {
			return config, fmt.Errorf("cannot read rule %s: %w", config.File, err)
		}
		config.RuleSource = define.NewRuleSource(body)
	}
	return config, nil
}

func csrInjectCode(config define.JavaScriptConfig) string {
	if !config.RuleSource.Present() {
		return ""
	}
	return jssdk.GenerateCSRInjectParser([]byte(config.RuleSource.Text()))
}

func ParsePageByBrowser(config define.JavaScriptConfig, container string, proxyAddr string, useMixParser bool) define.BodyParsed {
	return ParsePageByBrowserContext(context.Background(), config, container, proxyAddr, useMixParser)
}

func ParsePageByBrowserContext(ctx context.Context, config define.JavaScriptConfig, container string, proxyAddr string, useMixParser bool) define.BodyParsed {
	if err := ctx.Err(); err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), nil)
	}
	if err := ValidateReadySelector(config.ReadySelector); err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	var err error
	config, err = captureBrowserRuleContext(ctx, config)
	if err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	if proxyAddr == "" {
		proxyAddr = network.OptionsForConfigContext(ctx, config).Proxy
	}
	normalizedProxy, err := network.NormalizeProxy(proxyAddr)
	if err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_INIT_NETWORK_FAILED, err.Error(), time.Now(), nil)
	}
	config.Proxy = normalizedProxy
	proxyAddr = normalizedProxy
	key := browserCacheKeyContext(ctx, config, useMixParser)
	decode := func(data string) define.BodyParsed {
		document, err := decodeBrowserDocument(data)
		if err != nil {
			return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
		}
		if useMixParser {
			doc := define.RemoteBodySanitized{Body: document.Body, FinalURL: document.FinalURL, BaseURL: document.BaseURL, HTTPStatus: document.HTTPStatus}
			return ParseDataAndConfigBySSRContext(ctx, config, doc, "")
		}
		return processCSRItemsContext(ctx, config, document.Body, document.BaseURL)
	}
	timeout := define.HEADLESS_EXCUTE_TIMEOUT
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	sharedTimeout := fn.ExpireBySecond(timeout)
	cacheEnabled := cacher.IsEnabledContext(ctx) && !config.DisableCache
	validatePayload := func(encoded string) error {
		document, err := decodeBrowserDocument(encoded)
		if err != nil {
			return err
		}
		if useMixParser {
			if document.Body == "" {
				return fmt.Errorf("empty browser HTML document")
			}
			return nil
		}
		if parsed := parseCSRItems(document.Body); parsed.Code != define.ERROR_CODE_NULL {
			return fmt.Errorf("%s", parsed.Status)
		}
		return nil
	}
	data, err := browserData(ctx, key, cacheEnabled, sharedTimeout, config.Expire, func(work context.Context) (string, error) {
		return fetchBrowserData(work, config, container, proxyAddr, useMixParser)
	}, validatePayload)
	if err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), nil)
	}
	return decode(data)
}

// browserData shares only captured page data. Each caller decodes using its own
// context, so a departing caller cannot cancel the other callers' detail work.
func browserData(ctx context.Context, key string, cacheEnabled bool, timeout time.Duration, expire int, fetchData func(context.Context) (string, error), validators ...func(string) error) (string, error) {
	maxBytes := limits.MaxPageBytes
	if len(validators) > 0 {
		maxBytes = limits.MaxDocumentCacheBytes
	}
	validate := func(data string) error {
		if len(data) > maxBytes {
			return fmt.Errorf("browser result exceeds size limit")
		}
		for _, validator := range validators {
			if err := validator(data); err != nil {
				return err
			}
		}
		return nil
	}
	load := func(work context.Context) (string, error) {
		if cacheEnabled {
			if cached, found, err := cacher.GetWithinLimit(work, key, maxBytes); err == nil && found {
				if validate(cached) == nil {
					return cached, nil
				}
				_ = cacher.DeleteWithContext(work, key)
			}
		}
		if err := work.Err(); err != nil {
			return "", err
		}
		data, err := fetchData(work)
		if err != nil {
			return "", err
		}
		if err := validate(data); err != nil {
			return "", err
		}
		if err := work.Err(); err != nil {
			return "", err
		}
		if cacheEnabled {
			if expire <= 0 {
				expire = define.IN_MEMORY_EXPIRATION
			}
			if err := cacher.SetWithTTL(work, key, data, fn.ExpireBySecond(expire)); err != nil {
				logger.Instance.Warn("Unable to use cache")
			}
		}
		return data, nil
	}
	if !cacheEnabled {
		work, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return load(work)
	}
	// Application scope prevents one cache/runtime instance from joining another.
	data, err, shared := browserFlights.Do(ctx, cacher.Scope(ctx)+":"+network.Scope(ctx)+":"+key, timeout, load)
	if shared {
		telemetry.Default.Add("browser_shared_total", 1)
	}
	return data, err
}

func fetchBrowserData(ctx context.Context, config define.JavaScriptConfig, container, proxyAddr string, useMixParser bool) (string, error) {
	var document browserDocument
	err := withBrowserPage(ctx, container, proxyAddr, config.URL, config.Cookies, false, func(pageCtx context.Context) error {
		responses, stop, err := startMainDocumentResponses(pageCtx)
		if err != nil {
			return err
		}
		defer stop()
		response, err := chromedp.Run(pageCtx, chromedp.NavigateResponse(config.URL))
		if err != nil {
			return err
		}
		if response == nil || response.Status != http.StatusOK {
			if response == nil {
				return fmt.Errorf("browser navigation returned no HTTP document")
			}
			return fmt.Errorf("browser document returned HTTP %d", response.Status)
		}
		if err := chromedp.Do(pageCtx, chromedp.WaitReady(chromedp.CSS(browserReadySelector(config)))); err != nil {
			return err
		}
		expression := "(" + jssdk.TPL_MIX_JS + ")()"
		if !useMixParser {
			injectCode := csrInjectCode(config)
			if injectCode == "" {
				return fmt.Errorf("cannot read rule %s", config.File)
			}
			expression = "(" + injectCode + ")()"
		}
		before, err := responses.currentDocument(pageCtx)
		if err != nil {
			return err
		}
		if before.Status != http.StatusOK {
			return fmt.Errorf("browser document returned HTTP %d", before.Status)
		}
		document, err = chromedp.Run(pageCtx, chromedp.Evaluate[browserDocument](browserDocumentExpression(expression)))
		if err != nil {
			return err
		}
		after, err := responses.currentDocument(pageCtx)
		if err != nil {
			return err
		}
		if before.Loader != after.Loader {
			return fmt.Errorf("main document navigated during extraction")
		}
		document.HTTPStatus = after.Status
		if err := document.validate(); err != nil {
			return err
		}
		if !useMixParser {
			if parsed := parseCSRItems(document.Body); parsed.Code != define.ERROR_CODE_NULL {
				return fmt.Errorf("%s", parsed.Status)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return encodeBrowserDocument(document)
}

// Check the serialized value inside Chrome before CDP transfers it to Go.
func boundedBrowserExpression(expression string) string {
	return fmt.Sprintf(`(()=>{const value=(%s);if(typeof value!=="string")throw new Error("invalid browser result");if(value.length>%d||new TextEncoder().encode(value).length>%d)throw new Error("browser result exceeds size limit");return value;})()`, expression, limits.MaxPageBytes, limits.MaxPageBytes)
}

func ProxyPageByBrowser(url string, container string, proxyAddr string, cookies string) (string, error) {
	return ProxyPageByBrowserContext(context.Background(), url, container, proxyAddr, cookies)
}

func ProxyPageByBrowserContext(ctx context.Context, url, container, proxyAddr, cookies string) (string, error) {
	return ProxyPageByBrowserWithTimeout(ctx, url, container, proxyAddr, cookies, fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT))
}

func ProxyPageByBrowserWithTimeout(ctx context.Context, url, container, proxyAddr, cookies string, timeout time.Duration) (string, error) {
	if err := network.PublicURL(url); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var html string
	err := withBrowserPage(ctx, container, "", url, cookies, true, func(pageCtx context.Context) error {
		if err := chromedp.Do(pageCtx, chromedp.Navigate(url), chromedp.WaitReady(chromedp.CSS("body"))); err != nil {
			return err
		}
		var err error
		html, err = chromedp.Run(pageCtx, chromedp.Evaluate[string](boundedBrowserExpression("("+jssdk.TPL_MIX_JS+")()")))
		return err
	})
	return html, err
}

// browserReadySelector preserves delayed-content behavior by default. Rules can
// instead name a loaded/empty-state marker (for example body) when a ready page
// legitimately has no ListContainer matches.
func browserReadySelector(config define.JavaScriptConfig) string {
	if selector := strings.TrimSpace(config.ReadySelector); selector != "" {
		return selector
	}
	return strings.TrimSpace(config.ListContainer)
}
