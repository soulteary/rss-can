package network

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/soulteary/goquery"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func MixCodeStatus(code define.ErrorCode, desc string, detail any) (define.ErrorCode, string) {
	return code, fmt.Sprintf("%s: %v", desc, detail)
}

func HttpGet(url string, userAgent string) (code define.ErrorCode, status string, response *http.Response) {
	return HttpGetContext(context.Background(), url, userAgent)
}

func HttpGetContext(ctx context.Context, url string, userAgent string) (code define.ErrorCode, status string, response *http.Response) {
	client := &http.Client{Timeout: fn.ExpireBySecond(define.REQUEST_TIMEOUT)}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		code, status = MixCodeStatus(define.ERROR_CODE_INIT_NETWORK_FAILED, define.ERROR_STATUS_INIT_NETWORK_FAILED, err)
		return code, status, response
	}

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	} else {
		req.Header.Set("User-Agent", define.USER_AGENT)
	}

	response, err = client.Do(req)
	if err != nil {
		code, status = MixCodeStatus(define.ERROR_CODE_NETWORK, define.ERROR_STATUS_NETWORK, err)
		return code, status, response
	}

	return define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, response
}

func GetRemoteDocument(url string, charset string, expire int, disableCache bool) define.RemoteBodySanitized {
	return GetRemoteDocumentContext(context.Background(), url, charset, expire, disableCache)
}

// MaxDocumentBytes limits the decoded HTML retained for a single upstream page.
const MaxDocumentBytes = 10 << 20

func GetRemoteDocumentContext(ctx context.Context, url string, charset string, expire int, disableCache bool) define.RemoteBodySanitized {
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

	if cacher.IsEnable() && !disableCache {
		cache, found, err := cacher.GetWithStatus(ctx, url)
		if err == nil && found {
			logger.Instance.Debugln("Get remote document from cache", url)
			return define.MixupRemoteBodySanitized(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, now, cache)
		}
	}

	// todo change ua with config
	code, status, res := HttpGetContext(ctx, url, define.USER_AGENT)
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

	if cacher.IsEnable() && !disableCache {
		ttl := fn.ExpireBySecond(expire)
		if expire <= 0 {
			ttl = fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION)
		}
		err = cacher.SetWithTTL(ctx, url, buffer.String(), ttl)
		if err != nil {
			logger.Instance.Warn("Unable to use cache")
		}
	}
	return define.MixupRemoteBodySanitized(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, now, buffer.String())
}

func GetRemoteDocumentAsMarkdown(url string, selector string, charset string, expire int, disableCache bool) string {
	return GetRemoteDocumentAsMarkdownContext(context.Background(), url, selector, charset, expire, disableCache)
}

func GetRemoteDocumentAsMarkdownContext(ctx context.Context, url string, selector string, charset string, expire int, disableCache bool) string {
	doc := GetRemoteDocumentContext(ctx, url, charset, expire, disableCache)
	if doc.Body == "" {
		return ""
	}

	document, err := goquery.NewDocumentFromReader(strings.NewReader(doc.Body))
	if err != nil {
		return ""
	}

	// default selector use whole document body
	if selector == "" {
		selector = "body"
	}
	html, err := document.Find(selector).Html()
	if err != nil {
		return ""
	}

	return fn.Html2Md(html)
}
