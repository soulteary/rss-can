package fn_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestDetectContentEncoding(t *testing.T) {
	mockUTF8 := bytes.NewReader([]byte{0xE6, 0xB5, 0x8B, 0xE8, 0xAF, 0x95, 0x31, 0x32, 0x33, 0x34})
	encoding := fn.DetectContentEncoding(mockUTF8)
	if encoding != define.CHARSET_UTF8 {
		t.Fatal("DetectContentEncoding failed")
	}

	mockGBK := bytes.NewReader([]byte{0xB2, 0xE2, 0xCA, 0xD4, 0x31, 0x32, 0x33, 0x34})
	encoding = fn.DetectContentEncoding(mockGBK)
	if encoding != define.CHARSET_UTF8 {
		t.Fatal("DetectContentEncoding failed")
	}

	mockStr := strings.NewReader("test utf8 encoding")
	encoding = fn.DetectContentEncoding(mockStr)
	if encoding != define.CHARSET_UTF8 {
		t.Fatal("DetectContentEncoding failed")
	}
}

func streamToString(stream io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	return buf.String()
}

func TestDecodeHTMLBody(t *testing.T) {
	mockUTF8 := bytes.NewReader([]byte{0xE6, 0xB5, 0x8B, 0xE8, 0xAF, 0x95, 0x31, 0x32, 0x33, 0x34})
	buf, _ := fn.DecodeHTMLBody(mockUTF8, define.CHARSET_UTF8)
	ret := streamToString(buf)
	if ret != "测试1234" {
		t.Fatal("DecodeHTMLBody failed")
	}

	mockGBK := bytes.NewReader([]byte{0xB2, 0xE2, 0xCA, 0xD4, 0x31, 0x32, 0x33, 0x34})
	buf, _ = fn.DecodeHTMLBody(mockGBK, define.CHARSET_GBK)
	ret = streamToString(buf)
	if ret != "测试1234" {
		t.Fatal("DecodeHTMLBody failed")
	}

	// test not in encoding list
	mockKr := bytes.NewReader([]byte{0x3F, 0x3F, 0x31, 0x32, 0x33, 0x34})
	buf, _ = fn.DecodeHTMLBody(mockKr, "CP949")
	ret = streamToString(buf)
	if ret == "测试1234" {
		t.Fatal("DecodeHTMLBody failed")
	}

	// test auto detection
	buf, _ = fn.DecodeHTMLBody(mockUTF8, "")
	ret = streamToString(buf)
	if ret == "测试1234" {
		t.Fatal("DecodeHTMLBody failed")
	}
}
