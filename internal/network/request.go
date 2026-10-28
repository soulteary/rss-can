package network

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/soulteary/goquery"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/coalesce"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

func MixCodeStatus(code define.ErrorCode, desc string, detail any) (define.ErrorCode, string) {
	return code, fmt.Sprintf("%s: %v", desc, detail)
}

func HttpGet(url string, userAgent string) (code define.ErrorCode, status string, response *http.Response) {
	return HttpGetContext(context.Background(), url, userAgent)
}

func HttpGetContext(ctx context.Context, targetURL string, userAgent string, options ...RequestOptions) (code define.ErrorCode, status string, response *http.Response) {
	settings := configForContext(ctx)
	client := &http.Client{Timeout: settings.RequestTimeout}
	opts := requestOptions(options)
	if len(options) == 0 && scopedClient(ctx) != nil {
		opts.Proxy = settings.Proxy
	}
	proxy, err := NormalizeProxy(opts.Proxy)
	if err != nil {
		return define.ERROR_CODE_INIT_NETWORK_FAILED, "invalid proxy configuration", nil
	}
	var transport *http.Transport
	if proxy != "" {
		proxyURL, _ := url.Parse(proxy)
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(proxyURL)
		client.Transport = transport
	}
	if opts.Cookies != "" {
		origin, parseErr := url.Parse(opts.CookieURL)
		if parseErr != nil || origin.Host == "" {
			return define.ERROR_CODE_INIT_NETWORK_FAILED, "invalid cookie origin", nil
		}
		jar, _ := cookiejar.New(nil)
		cookieRequest := &http.Request{Header: http.Header{"Cookie": []string{opts.Cookies}}}
		cookies := cookieRequest.Cookies()
		for _, cookie := range cookies {
			cookie.Path = "/"
		}
		jar.SetCookies(origin, cookies)
		client.Jar = jar
	}
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		code, status = MixCodeStatus(define.ERROR_CODE_INIT_NETWORK_FAILED, define.ERROR_STATUS_INIT_NETWORK_FAILED, err)
		return code, status, response
	}

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	} else {
		req.Header.Set("User-Agent", settings.UserAgent)
	}

	release, err := upstreamBudget.acquire(ctx)
	if err != nil {
		if transport != nil {
			transport.CloseIdleConnections()
		}
		failureCode := define.ERROR_CODE_NETWORK
		if errors.Is(err, ErrFetchOverloaded) {
			failureCode = define.ERROR_CODE_OVERLOADED
		}
		code, status = MixCodeStatus(failureCode, define.ERROR_STATUS_NETWORK, err)
		return code, status, nil
	}
	response, err = client.Do(req)
	if transport != nil {
		if response != nil {
			response.Body = &transportBody{ReadCloser: response.Body, transport: transport}
		} else {
			transport.CloseIdleConnections()
		}
	}
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		release()
		code, status = MixCodeStatus(define.ERROR_CODE_NETWORK, define.ERROR_STATUS_NETWORK, err)
		return code, status, response
	}
	response.Body = &budgetBody{ReadCloser: response.Body, release: release}

	return define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, response
}

func GetRemoteDocument(url string, charset string, expire int, disableCache bool) define.RemoteBodySanitized {
	return GetRemoteDocumentContext(context.Background(), url, charset, expire, disableCache)
}

// MaxDocumentBytes limits the decoded HTML retained for a single upstream page.
const MaxDocumentBytes = limits.MaxPageBytes

var documentFlights coalesce.Group[define.RemoteBodySanitized]

func GetRemoteDocumentContext(ctx context.Context, url string, charset string, expire int, disableCache bool, options ...RequestOptions) define.RemoteBodySanitized {
	if client := scopedClient(ctx); client != nil && len(options) == 0 {
		options = []RequestOptions{{Proxy: client.config.Proxy}}
	}
	if disableCache || !cacher.IsEnabledContext(ctx) {
		return getRemoteDocument(ctx, url, charset, expire, disableCache, options...)
	}
	key := documentKey(url, charset, expire, options...)
	flights := &documentFlights
	if client := scopedClient(ctx); client != nil {
		flights = &client.flights
	}
	key = cacher.Scope(ctx) + ":" + configForContext(ctx).UserAgent + ":" + key
	result, err, shared := flights.Do(ctx, key, configForContext(ctx).ServerTimeout, func(work context.Context) (define.RemoteBodySanitized, error) {
		return getRemoteDocument(work, url, charset, expire, false, options...), nil
	})
	if shared {
		telemetry.Default.Add("fetch_shared_total", 1)
	}
	if err != nil {
		return define.MixupRemoteBodySanitized(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), "")
	}
	return result
}

func documentKey(url, charset string, expire int, options ...RequestOptions) string {
	opts := requestOptions(options)
	return fmt.Sprintf("document:v3:%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%s\x00%s", url, charset, expire, opts.CookieURL, opts.Cookies, opts.Proxy))))
}

func getRemoteDocument(ctx context.Context, url string, charset string, expire int, disableCache bool, options ...RequestOptions) (result define.RemoteBodySanitized) {
	key := documentKey(url, charset, expire, options...)
	if scopedClient(ctx) != nil {
		key += fmt.Sprintf(":ua:%x", sha256.Sum256([]byte(configForContext(ctx).UserAgent)))
	}
	var code define.ErrorCode
	var status string
	var now = time.Now()
	if err := ctx.Err(); err != nil {
		return define.MixupRemoteBodySanitized(define.ERROR_CODE_NETWORK, err.Error(), now, "")
	}

	defer func() { // log when error
		if code != define.ERROR_CODE_NULL {
			logger.Instance.Warnf("get url:%v, code:%v status:%s", url, code, status)
		}
	}()

	if cacher.IsEnabledContext(ctx) && !disableCache {
		cache, found, err := cacher.GetWithinLimit(ctx, key, limits.MaxDocumentCacheBytes)
		if err == nil && found {
			var cached define.RemoteBodySanitized
			if json.Unmarshal([]byte(cache), &cached) == nil && validCachedDocument(cached) {
				logger.Instance.Debugln("Get remote document from cache", url)
				return cached
			}
			_ = cacher.DeleteWithContext(ctx, key)
		}
	}

	if err := ctx.Err(); err != nil {
		return define.MixupRemoteBodySanitized(define.ERROR_CODE_NETWORK, err.Error(), now, "")
	}
	began := time.Now()
	telemetry.Default.Add("fetch_requests_total", 1)
	defer func() {
		telemetry.Default.Observe("fetch_duration_seconds", time.Since(began))
		if result.Code != define.ERROR_CODE_NULL {
			telemetry.Default.Add("fetch_failures_total", 1)
		}
	}()
	// todo change ua with config
	code, status, res := HttpGetContext(ctx, url, "", options...)
	if code != define.ERROR_CODE_NULL {
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		code, status = MixCodeStatus(define.ERROR_CODE_API_NOT_READY, define.ERROR_STATUS_API_NOT_READY, res.StatusCode)
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}

	bodyParsed, err := fn.DecodeHTMLBody(res.Body, charset)
	if err != nil {
		code, status = MixCodeStatus(define.ERROR_CODE_DECODE_CAHRSET_FAILED, define.ERROR_STATUS_DECODE_CAHRSET_FAILED, err)
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}

	buffer := new(bytes.Buffer)
	if _, err := buffer.ReadFrom(io.LimitReader(bodyParsed, MaxDocumentBytes+1)); err != nil {
		code, status = MixCodeStatus(define.ERROR_CODE_NETWORK, define.ERROR_STATUS_NETWORK, err)
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}
	if buffer.Len() > MaxDocumentBytes {
		return define.MixupRemoteBodySanitized(define.ERROR_CODE_NETWORK, "upstream document exceeds 10 MiB", now, "")
	}
	if err := ctx.Err(); err != nil {
		return define.MixupRemoteBodySanitized(define.ERROR_CODE_NETWORK, err.Error(), now, "")
	}

	result = define.MixupRemoteBodySanitized(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, now, buffer.String())
	result.FinalURL = url
	if res.Request != nil && res.Request.URL != nil {
		result.FinalURL = res.Request.URL.String()
	}
	result.HTTPStatus = res.StatusCode
	if cacher.IsEnabledContext(ctx) && !disableCache {
		ttl := fn.ExpireBySecond(expire)
		if expire <= 0 {
			ttl = configForContext(ctx).DefaultTTL
		}
		encoded, encodeErr := json.Marshal(result)
		if encodeErr != nil || len(encoded) > limits.MaxDocumentCacheBytes || !validCachedDocument(result) {
			return result
		}
		err = cacher.SetWithTTL(ctx, key, string(encoded), ttl)
		if err != nil {
			logger.Instance.Warn("Unable to use cache")
		}
	}
	return result
}

func GetRemoteDocumentAsMarkdown(url string, selector string, charset string, expire int, disableCache bool) string {
	return GetRemoteDocumentAsMarkdownContext(context.Background(), url, selector, charset, expire, disableCache)
}

// GetRemoteDocumentAsMarkdownContext is a compatibility helper. Feed generation
// uses GetRemoteMarkdownContext so upstream and extraction errors remain visible.
func GetRemoteDocumentAsMarkdownContext(ctx context.Context, url string, selector string, charset string, expire int, disableCache bool) string {
	return GetRemoteMarkdownContext(ctx, url, selector, charset, expire, disableCache).Body
}

// GetRemoteMarkdownContext preserves failures instead of treating them as empty content.
func GetRemoteMarkdownContext(ctx context.Context, url, selector, charset string, expire int, disableCache bool, options ...RequestOptions) define.RemoteBodySanitized {
	doc := GetRemoteDocumentContext(ctx, url, charset, expire, disableCache, options...)
	if doc.Code != define.ERROR_CODE_NULL {
		return doc
	}
	fail := func(status string) define.RemoteBodySanitized {
		return define.MixupRemoteBodySanitized(define.ERROR_CODE_PARSE_CONTENT_FAILED, status, doc.Date, "")
	}
	if doc.Body == "" {
		return fail("empty detail response")
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(doc.Body))
	if err != nil {
		return fail("cannot parse detail document")
	}
	if selector == "" {
		selector = "body"
	}
	selection := document.Find(selector)
	if selection.Length() == 0 {
		return fail("detail selector matched no elements")
	}
	doc.BaseURL = detailDocumentBase(doc, document)
	if err := normalizeDetailURLs(ctx, selection, doc.BaseURL); err != nil {
		return fail(err.Error())
	}
	html, err := selection.Html()
	if err != nil {
		return fail("cannot extract detail HTML")
	}
	if len(html) > limits.MaxPageBytes {
		return fail("detail HTML exceeds content size limit")
	}
	doc.Body = fn.Html2Md(html)
	if len(doc.Body) > limits.MaxFeedBytes {
		return fail("detail Markdown exceeds content size limit")
	}
	if err := ctx.Err(); err != nil {
		return fail(err.Error())
	}
	return doc
}

func validDocumentURL(raw string) bool {
	if len(raw) > limits.MaxDocumentURLBytes {
		return false
	}
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Hostname() != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
func validCachedDocument(document define.RemoteBodySanitized) bool {
	return document.Code == define.ERROR_CODE_NULL && document.HTTPStatus == http.StatusOK && len(document.Body) <= MaxDocumentBytes && validDocumentURL(document.FinalURL) && (document.BaseURL == "" || validDocumentURL(document.BaseURL))
}
