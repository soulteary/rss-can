package charset

import (
	"bufio"
	"io"

	"github.com/soulteary/RSS-Can/internal/define"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/htmlindex"
)

// According to the returned content, automatically determine the content encoding format
func DetectContentCharset(body io.Reader) string {
	r := bufio.NewReader(body)
	if data, err := r.Peek(1024); err == nil {
		if _, name, ok := charset.DetermineEncoding(data, ""); ok {
			return name
		}
	}
	return define.DEFAULT_DOCUMENT_CHARSET
}

const CHARSET_GBK = "gbk"
const CHARSET_GB2312 = "gb2312"
const CHARSET_UTF8 = "utf-8"

// decodeHTMLBody returns an decoding reader of the html Body for the specified `charset`
// If `charset` is empty, decodeHTMLBody tries to guess the encoding from the content
func DecodeHTMLBody(body io.Reader, charset string) (io.Reader, error) {
	if charset == "" {
		charset = DetectContentCharset(body)
	} else {
		// use UTF-8 as fallback
		// TODO maybe support more preset types
		if !(charset == CHARSET_UTF8 || charset == CHARSET_GB2312 || charset == CHARSET_GBK) {
			charset = define.DEFAULT_DOCUMENT_CHARSET
		}
	}
	e, err := htmlindex.Get(charset)
	if err != nil {
		return nil, err
	}
	if name, _ := htmlindex.Name(e); name != define.DEFAULT_DOCUMENT_CHARSET {
		body = e.NewDecoder().Reader(body)
	}
	return body, nil
}
