package network

import (
	"fmt"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
	Charset "github.com/soulteary/RSS-Can/internal/charset"
	"github.com/soulteary/RSS-Can/internal/define"
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

func mixData(code define.ErrorCode, status string, date time.Time, data []define.Item) (response define.Response) {
	response.Code = code
	response.Status = status
	response.Date = date
	response.Data = data
	return response
}

func GetRemoteDocument(url string, charset string, callback func(document *goquery.Document) []define.Item) (response define.Response) {
	var code define.ErrorCode
	var status string
	var document *goquery.Document
	var now = time.Now()

	code, status, res := Get(url, define.GLOBAL_USER_AGENT)
	if code != define.ERROR_CODE_NULL {
		return mixData(code, status, now, nil)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		code = define.ERROR_CODE_API_NOT_READY
		status = fmt.Sprintf("%s: %d %s", define.ERROR_STATUS_API_NOT_READY, res.StatusCode, res.Status)
		return mixData(code, status, now, nil)
	}

	bodyParsed, err := Charset.DecodeHTMLBody(res.Body, charset)
	if err != nil {
		code = define.ERROR_CODE_DECODE_CAHRSET_FAILED
		status = fmt.Sprintf("%s: %s", define.ERROR_STATUS_DECODE_CAHRSET_FAILED, fmt.Errorf("%w", err))
		return mixData(code, status, now, nil)
	}

	document, err = goquery.NewDocumentFromReader(bodyParsed)
	if err != nil {
		code = define.ERROR_CODE_PARSE_CONTENT_FAILED
		status = fmt.Sprintf("%s: %s", define.ERROR_STATUS_PARSE_CONTENT_FAILED, fmt.Errorf("%w", err))
		return mixData(code, status, now, nil)
	}

	items := callback(document)
	code = define.ERROR_CODE_NULL
	status = define.ERROR_STATUS_NULL

	return mixData(code, status, now, items)
}
