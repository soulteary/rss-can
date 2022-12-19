package fn

import (
	"bufio"
	"io"

	"github.com/soulteary/RSS-Can/internal/define"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/htmlindex"
)

// According to the returned content, automatically determine the content encoding format
func DetectContentEncoding(body io.Reader) string {
	r := bufio.NewReader(body)
	if data, err := r.Peek(1024); err == nil {
		if _, name, ok := charset.DetermineEncoding(data, ""); ok {
			return name
		}
	}
	return define.DEFAULT_DOCUMENT_CHARSET
}

// decodeHTMLBody returns an decoding reader of the html Body for the specified `charset`
// If `charset` is empty, decodeHTMLBody tries to guess the encoding from the content
func DecodeHTMLBody(body io.Reader, charset string) (io.Reader, error) {
	if charset == "" {
		charset = DetectContentEncoding(body)
	} else {
		if !(charset == define.CHARSET_UTF8 || charset == define.CHARSET_GB2312 || charset == define.CHARSET_GBK || charset == define.CHARSET_GB18030) {
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
