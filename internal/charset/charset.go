package charset

import (
	"bufio"
	"io"

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
	return "utf-8"
}

// decodeHTMLBody returns an decoding reader of the html Body for the specified `charset`
// If `charset` is empty, decodeHTMLBody tries to guess the encoding from the content
func DecodeHTMLBody(body io.Reader, charset string) (io.Reader, error) {
	if charset == "" {
		charset = DetectContentCharset(body)
	}
	e, err := htmlindex.Get(charset)
	if err != nil {
		return nil, err
	}
	if name, _ := htmlindex.Name(e); name != "utf-8" {
		body = e.NewDecoder().Reader(body)
	}
	return body, nil
}
