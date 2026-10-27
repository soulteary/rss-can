package parser

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
)

// Bound browser work, including connection, navigation and script evaluation.
var browserSlots = make(chan struct{}, 4)

func GetDataAndConfigByCSR(config define.JavaScriptConfig, container string, proxyAddr string) define.BodyParsed {
	return ParsePageByGoRod(config, container, proxyAddr, false)
}

func resolveBrowserURL(ctx context.Context, address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = define.DEFAULT_HEADLESS_SERVER
	}
	if _, err := strconv.Atoi(strings.TrimPrefix(address, ":")); err == nil {
		address = "127.0.0.1:" + strings.TrimPrefix(address, ":")
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	u, err := url.Parse(address)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "ws", "wss":
		if u.Path != "" && u.Path != "/" {
			return u.String(), nil
		}
		if u.Scheme == "ws" {
			u.Scheme = "http"
		} else {
			u.Scheme = "https"
		}
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported browser endpoint scheme")
	}
	u.Path = "/json/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("browser discovery returned %d", res.StatusCode)
	}
	var info struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.URL == "" {
		return "", fmt.Errorf("browser discovery returned no websocket URL")
	}
	websocketURL, err := url.Parse(info.URL)
	if err != nil {
		return "", err
	}
	if websocketURL.Scheme != "ws" && websocketURL.Scheme != "wss" {
		return "", fmt.Errorf("invalid browser websocket URL")
	}
	// Remote containers can advertise localhost. Keep the configured authority.
	websocketURL.Host = u.Host
	if u.Scheme == "https" {
		websocketURL.Scheme = "wss"
	}
	return websocketURL.String(), nil
}

// browserDialer bounds the websocket handshake, which Rod's websocket client
// otherwise performs without a socket deadline. The caller owns the connection.
type browserDialer struct {
	conn   net.Conn
	secure bool
}

func (d *browserDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var err error
	address = browserDialAddress(address, d.secure)
	if d.secure {
		dialer := &tls.Dialer{}
		d.conn, err = dialer.DialContext(ctx, network, address)
	} else {
		dialer := &net.Dialer{}
		d.conn, err = dialer.DialContext(ctx, network, address)
	}
	if err != nil {
		return nil, err
	}
	if err := d.conn.SetDeadline(deadline(ctx)); err != nil {
		_ = d.conn.Close()
		return nil, err
	}
	return d.conn, nil
}

func browserDialAddress(address string, secure bool) string {
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	port := "80"
	if secure {
		port = "443"
	}
	return net.JoinHostPort(strings.Trim(address, "[]"), port)
}

// withRodPage owns an isolated browser context and its connection. It never
// closes the shared remote browser. Cleanup also runs after panics or timeouts.
func withRodPage(ctx context.Context, container, proxyAddr, target, cookies string, publicOnly bool, action func(*rod.Page)) error {
	select {
	case browserSlots <- struct{}{}:
		defer func() { <-browserSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return rod.Try(func() {
		var address string
		if define.CSR_DEBUG && !publicOnly {
			l := launcher.New().Context(ctx).Headless(false).Devtools(true)
			if proxyAddr != "" {
				l.Set(flags.ProxyServer, proxyAddr)
			}
			defer l.Cleanup()
			defer l.Kill()
			address = l.MustLaunch()
		} else {
			var err error
			address, err = resolveBrowserURL(ctx, container)
			if err != nil {
				panic(err)
			}
		}
		dialer := &browserDialer{secure: strings.HasPrefix(address, "wss://")}
		defer func() {
			if dialer.conn != nil {
				_ = dialer.conn.Close()
			}
		}()
		ws := &cdp.WebSocket{Dialer: dialer}
		if err := ws.Connect(ctx, address, nil); err != nil {
			panic(err)
		}
		if err := dialer.conn.SetDeadline(time.Time{}); err != nil {
			panic(err)
		}
		browser := rod.New().Client(cdp.New().Start(ws)).Context(ctx)
		if err := browser.Connect(); err != nil {
			panic(err)
		}
		// Dispose on connection loss even if creation succeeds remotely after
		// the command times out locally and its context ID is never received.
		isolated := browser.Context(ctx)
		created, err := (proto.TargetCreateBrowserContext{DisposeOnDetach: true}).Call(isolated)
		if err != nil {
			panic(err)
		}
		isolated.BrowserContextID = created.BrowserContextID
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = isolated.Context(cleanupCtx).Close()
		}()
		if define.CSR_IGNORE_CERT_ERRORS && !publicOnly {
			isolated.MustIgnoreCertErrors(true)
		}
		page := isolated.MustPage()
		page.MustEvalOnNewDocument(jssdk.CSR_NO_OP_ALERT)
		if publicOnly {
			if err := (proto.NetworkSetBlockedURLs{Urls: []string{"ws://*", "wss://*", "file://*", "ftp://*"}}).Call(page); err != nil {
				panic(err)
			}
			if err := (proto.NetworkSetBypassServiceWorker{Bypass: true}).Call(page); err != nil {
				panic(err)
			}
		}
		router := page.HijackRequests()
		client := network.PublicHTTPClient(time.Until(deadline(ctx)))
		router.MustAdd("*", func(h *rod.Hijack) {
			resource := h.Request.Type()
			if resource == proto.NetworkResourceTypeImage || resource == proto.NetworkResourceTypeMedia || resource == proto.NetworkResourceTypeFont {
				h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
				return
			}
			if publicOnly {
				h.Request.SetContext(ctx)
				if err := network.PublicURL(h.Request.URL().String()); err != nil {
					h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
					return
				}
				if err := h.LoadResponse(client, true); err != nil {
					h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
				}
				return
			}
			h.ContinueRequest(&proto.FetchContinueRequest{})
		})
		defer router.Stop()
		go router.Run()
		if cookies != "" {
			// Scope rule cookies to the rule's actual host and never write them to logs.
			response := &http.Response{Header: http.Header{"Set-Cookie": nil}}
			u, err := url.Parse(target)
			if err != nil {
				panic(err)
			}
			for _, raw := range strings.Split(cookies, ";") {
				response.Header.Set("Set-Cookie", strings.TrimSpace(raw))
				for _, c := range response.Cookies() {
					page.MustSetCookies(&proto.NetworkCookieParam{Name: c.Name, Value: c.Value, URL: u.Scheme + "://" + u.Host + "/"})
				}
			}
		}
		action(page)
	})
}

func deadline(ctx context.Context) time.Time {
	d, ok := ctx.Deadline()
	if !ok {
		return time.Now().Add(5 * time.Second)
	}
	return d
}

func GetCSRInjectCode(file string) string {
	jsRule := fn.GetFileContent(file)
	if jsRule == nil {
		return ""
	}
	return jssdk.GenerateCSRInjectParser(jsRule)
}

func parseCSRItems(data string) define.BodyParsed {
	var items []define.InfoItem
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_PARSE_CONTENT_FAILED, err.Error(), time.Now(), nil)
	}
	return define.MixupBodyParsed(define.ERROR_CODE_NULL, define.ERROR_STATUS_NULL, time.Now(), items)
}

func browserCacheKey(config define.JavaScriptConfig, useMixParser bool) string {
	// Separate browser JSON/HTML from SSR HTML, and isolate rules, cookies and
	// parser settings. Include the rule bytes so edits cannot reuse old results.
	encoded, _ := json.Marshal(config)
	rule := fn.GetFileContent(config.File)
	sum := sha256.Sum256(append(encoded, rule...))
	return fmt.Sprintf("browser:v2:%t:%x", useMixParser, sum)
}

func ParsePageByGoRod(config define.JavaScriptConfig, container string, proxyAddr string, useMixParser bool) define.BodyParsed {
	key := browserCacheKey(config, useMixParser)
	decode := func(data string) define.BodyParsed {
		if useMixParser {
			return ParseDataAndConfigBySSR(config, define.RemoteBodySanitized{}, data)
		}
		return parseCSRItems(data)
	}
	if cacher.IsEnable() && !config.DisableCache {
		if cached, err := cacher.Get(key); err == nil && cached != "" {
			return decode(cached)
		}
	}
	timeout := define.HEADLESS_EXCUTE_TIMEOUT
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), fn.ExpireBySecond(timeout))
	defer cancel()
	var data string
	err := withRodPage(ctx, container, proxyAddr, config.URL, config.Cookies, false, func(page *rod.Page) {
		page.MustNavigate(config.URL).MustWaitLoad().MustElement(config.ListContainer)
		if useMixParser {
			data = fmt.Sprint(page.MustEval(jssdk.TPL_MIX_JS))
			return
		}
		injectCode := GetCSRInjectCode(config.File)
		if injectCode == "" {
			panic(fmt.Errorf("cannot read rule %s", config.File))
		}
		data = fmt.Sprint(page.MustEval(injectCode))
	})
	if err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_NETWORK, err.Error(), time.Now(), nil)
	}
	result := decode(data)
	if result.Code == define.ERROR_CODE_NULL && cacher.IsEnable() && !config.DisableCache {
		if err := cacher.Set(key, data); err != nil {
			logger.Instance.Warn("Unable to use cache")
		} else {
			expire := config.Expire
			if expire <= 0 {
				expire = define.IN_MEMORY_EXPIRATION
			}
			_ = cacher.Expire(key, fn.ExpireBySecond(expire))
		}
	}
	return result
}

func ProxyPageByGoRod(url string, container string, proxyAddr string, cookies string) (string, error) {
	if err := network.PublicURL(url); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT))
	defer cancel()
	var html string
	err := withRodPage(ctx, container, "", url, "", true, func(page *rod.Page) {
		page.MustNavigate(url).MustWaitLoad().MustElement("body")
		html = fmt.Sprint(page.MustEval(jssdk.TPL_MIX_JS))
	})
	return html, err
}
