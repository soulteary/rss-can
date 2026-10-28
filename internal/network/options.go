package network

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
)

// RequestOptions is copied through listing, pagination and detail requests.
// CookieURL anchors host-only cookies to the rule's original origin.
type RequestOptions struct {
	Cookies   string
	CookieURL string
	Proxy     string
}

func OptionsForConfig(config define.JavaScriptConfig) RequestOptions {
	proxy := strings.TrimSpace(config.Proxy)
	if proxy == "" {
		proxy = define.PROXY_SERVER
	}
	return RequestOptions{Cookies: config.Cookies, CookieURL: config.URL, Proxy: proxy}
}

func requestOptions(options []RequestOptions) RequestOptions {
	if len(options) > 0 {
		return options[0]
	}
	return RequestOptions{}
}

// NormalizeProxy supports legacy host:port values and explicit HTTP(S) URLs.
func NormalizeProxy(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("proxy must be an HTTP(S) URL or host:port")
	}
	return u.String(), nil
}

type transportBody struct {
	io.ReadCloser
	transport *http.Transport
}

func (b *transportBody) Close() error {
	err := b.ReadCloser.Close()
	b.transport.CloseIdleConnections()
	return err
}
