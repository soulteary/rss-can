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

func Get(url string, userAgent string) (code define.ErrorCode, status string, response *http.Response) {
	client := &http.Client{Timeout: fn.I2T(define.REQUEST_TIMEOUT) * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		code = define.ERROR_CODE_INIT_NETWORK_FAILED
		status = fmt.Sprintf("%s: %s", define.ERROR_STATUS_INIT_NETWORK_FAILED, fmt.Errorf("%w", err))
		return code, status, response
	}

	req.Header.Set("User-Agent", userAgent)

	response, err = client.Do(req)
	if err != nil {
		code = define.ERROR_CODE_NETWORK
		status = fmt.Sprintf("%s: %s", define.ERROR_STATUS_NETWORK, fmt.Errorf("%w", err))
		return code, status, response
	}

	code = define.ERROR_CODE_NULL
	status = define.ERROR_STATUS_NULL
	return code, status, response
}

func GetRemoteDocument(url string, charset string, expire time.Duration, disableCache bool) define.RemoteBodySanitized {
	var code define.ErrorCode
	var status string
	var now = time.Now()

	if cacher.IsEnable() && !disableCache {
		cache, err := cacher.Get(url)
		if err == nil && cache != "" {
			logger.Instance.Debugln("Get remote document from cache", url)
			code = define.ERROR_CODE_NULL
			status = define.ERROR_STATUS_NULL
			return define.MixupRemoteBodySanitized(code, status, now, cache)
		}
	}

	code, status, res := Get(url, define.USER_AGENT)
	if code != define.ERROR_CODE_NULL {
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		code = define.ERROR_CODE_API_NOT_READY
		status = fmt.Sprintf("%s: %d %s", define.ERROR_STATUS_API_NOT_READY, res.StatusCode, res.Status)
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}

	bodyParsed, err := fn.DecodeHTMLBody(res.Body, charset)
	if err != nil {
		code = define.ERROR_CODE_DECODE_CAHRSET_FAILED
		status = fmt.Sprintf("%s: %s", define.ERROR_STATUS_DECODE_CAHRSET_FAILED, fmt.Errorf("%w", err))
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}

	code = define.ERROR_CODE_NULL
	status = define.ERROR_STATUS_NULL
	buffer := new(bytes.Buffer)
	buffer.ReadFrom(bodyParsed)

	if cacher.IsEnable() && !disableCache {
		err = cacher.Set(url, buffer.String())
		if err != nil {
			logger.Instance.Warn("Unable to use cache")
		} else {
			if expire > 0 {
				cacher.Expire(url, expire)
			} else {
				cacher.Expire(url, fn.I2T(define.IN_MEMORY_EXPIRATION)*time.Second)
			}
		}
	}
	return define.MixupRemoteBodySanitized(code, status, now, buffer.String())
}

func GetRemoteDocumentAsMarkdown(url string, selector string, charset string, expire time.Duration, disableCache bool) string {
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

	md, err := fn.Html2Md(html)
	if err != nil {
		return ""
	}

	return md
}
