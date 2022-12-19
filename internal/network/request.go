package network

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/soulteary/RSS-Can/internal/charset"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

func Get(url string, userAgent string) (code define.ErrorCode, status string, response *http.Response) {
	client := &http.Client{Timeout: define.GLOBAL_REQ_TIMEOUT}
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

func GetRemoteDocument(url string, docCharset string) define.RemoteBodySanitized {
	var code define.ErrorCode
	var status string
	var now = time.Now()

	code, status, res := Get(url, define.GLOBAL_USER_AGENT)
	if code != define.ERROR_CODE_NULL {
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		code = define.ERROR_CODE_API_NOT_READY
		status = fmt.Sprintf("%s: %d %s", define.ERROR_STATUS_API_NOT_READY, res.StatusCode, res.Status)
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}

	bodyParsed, err := charset.DecodeHTMLBody(res.Body, docCharset)
	if err != nil {
		code = define.ERROR_CODE_DECODE_CAHRSET_FAILED
		status = fmt.Sprintf("%s: %s", define.ERROR_STATUS_DECODE_CAHRSET_FAILED, fmt.Errorf("%w", err))
		return define.MixupRemoteBodySanitized(code, status, now, "")
	}

	code = define.ERROR_CODE_NULL
	status = define.ERROR_STATUS_NULL
	buffer := new(bytes.Buffer)
	buffer.ReadFrom(bodyParsed)
	return define.MixupRemoteBodySanitized(code, status, now, buffer.String())
}

func GetRemoteDocumentAsMarkdown(url string, selector string, docCharset string) string {
	doc := GetRemoteDocument(url, docCharset)
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
