package network

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func MixCodeStatus(code define.ErrorCode, desc string, detail any) (define.ErrorCode, string) {
	return code, fmt.Sprintf("%s: %v", desc, detail)
}

func HttpGet(url string, userAgent string) (code define.ErrorCode, status string, response *http.Response) {
	client := &http.Client{Timeout: fn.I2T(define.REQUEST_TIMEOUT) * time.Second}
	req, err := http.NewRequest("GET", url, nil)
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
	var code define.ErrorCode
	var status string
	var now = time.Now()

	if cacher.IsEnable() && !disableCache {
		cache, err := cacher.Get(url)
		if err == nil && cache != "" {
			logger.Instance.Debugln("Get remote document from cache", url)
			return define.MixupRemoteBodySanitized(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, now, cache)
		}
	}

	// todo change ua with config
	code, status, res := HttpGet(url, define.USER_AGENT)
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
	buffer.ReadFrom(bodyParsed)

	if cacher.IsEnable() && !disableCache {
		err = cacher.Set(url, buffer.String())
		if err != nil {
			logger.Instance.Warn("Unable to use cache")
		} else {
			if expire > 0 {
				cacher.Expire(url, fn.I2T(expire)*time.Second)
			} else {
				cacher.Expire(url, fn.I2T(define.IN_MEMORY_EXPIRATION)*time.Second)
			}
		}
	}
	return define.MixupRemoteBodySanitized(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, now, buffer.String())
}

func GetRemoteDocumentAsMarkdown(url string, selector string, charset string, expire int, disableCache bool) string {
	doc := GetRemoteDocument(url, charset, expire, disableCache)
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
