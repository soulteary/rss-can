package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func GetDataAndConfigByCSR(config define.JavaScriptConfig, container string, proxyAddr string) (result define.BodyParsed) {
	return ParsePageByGoRod(config, container, proxyAddr, false)
}

func GetRodPageObject(container string, proxyAddr string, cookies string) *rod.Page {
	var browser *rod.Browser
	var page *rod.Page

	if define.CSR_DEBUG {
		l := launcher.New().Headless(false).Devtools(true)
		if proxyAddr != "" {
			l.Set(flags.ProxyServer, proxyAddr)
		}
		browser = rod.New().ControlURL(l.MustLaunch()).Trace(true).SlowMotion(fn.ExpireBySecond(define.HEADLESS_SLOW_MOTION))
		launcher.Open(browser.ServeMonitor(""))
	} else {
		browser = rod.New().ControlURL(launcher.MustResolveURL(container))
	}
	browser = browser.MustConnect()

	if define.CSR_IGNORE_CERT_ERRORS {
		browser = browser.MustIgnoreCertErrors(true)
	}

	if define.CSR_INCOGNITO {
		page = browser.MustIncognito().MustPage()
	} else {
		page = browser.MustPage()
	}

	// avoid data process hang due to pop-up windows
	page.MustEvalOnNewDocument(jssdk.CSR_NO_OP_ALERT)

	if cookies == "" {
		router := page.HijackRequests()
		frugal := func(ctx *rod.Hijack) {
			resType := ctx.Request.Type()
			if resType == proto.NetworkResourceTypeImage || resType == proto.NetworkResourceTypeMedia || resType == proto.NetworkResourceTypeFont {
				ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			} else {
				ctx.ContinueRequest(&proto.FetchContinueRequest{})
			}
		}
		router.MustAdd("*", frugal)
		go router.Run()
	}

	if cookies != "" {
		cookieList := strings.SplitN(cookies, ";", -1)
		var splitCookie = regexp.MustCompile(`(?m)^(.+?=)(.+)$`)
		expr := proto.TimeSinceEpoch(time.Now().Add(180 * 24 * time.Hour).Unix())

		for _, cookie := range cookieList {
			cookie = strings.TrimSpace(cookie)
			for _, match := range splitCookie.FindAllStringSubmatch(cookie, -1) {
				if len(match) == 3 {
					key := strings.TrimRight(match[1], "=")
					value := match[2]

					cookieItem := &proto.NetworkCookieParam{
						Name:    key,
						Value:   value,
						Domain:  ".linkedin.com",
						Expires: expr,
					}

					page.MustSetCookies(cookieItem)
					fmt.Println(key, value)
				}
			}
		}
	}

	// read network values
	for i, cookie := range page.MustCookies() {
		fmt.Printf("chrome cookie %d: %+v", i, cookie)
	}

	return page
}

func GetCSRInjectCode(file string) string {
	jsRule := fn.GetFileContent(file)
	if jsRule == nil {
		logger.Instance.Errorf("Open rule failed %v")
		return ""
	}
	return jssdk.GenerateCSRInjectParser(jsRule)
}

func parseHTMLtoItems(data string) []define.InfoItem {
	var items []define.InfoItem
	err := json.Unmarshal([]byte(data), &items)
	if err != nil {
		logger.Instance.Errorf("Parsing HTML to Items failed: %v", err)
		return items
	}
	return items
}

func ParsePageByGoRod(config define.JavaScriptConfig, container string, proxyAddr string, useMixParser bool) (result define.BodyParsed) {
	if cacher.IsEnable() && !config.DisableCache {
		cache, err := cacher.Get(config.URL)
		if err == nil && cache != "" {
			logger.Instance.Debugln("Get remote document from cache")
			code := define.ERROR_CODE_NULL
			status := define.ERROR_STATUS_NULL
			items := parseHTMLtoItems(cache)
			now := time.Now()
			return define.MixupBodyParsed(code, status, now, items)
		}
	}

	page := GetRodPageObject(container, proxyAddr, config.Cookies)

	// TODO support pager config

	timeout := define.HEADLESS_EXCUTE_TIMEOUT
	if config.Timeout > 0 {
		timeout = config.Timeout
	}

	page.
		Timeout(fn.ExpireBySecond(timeout)).
		MustNavigate(config.URL).
		MustWaitLoad().
		MustElement(config.ListContainer).
		CancelTimeout()

	if useMixParser {
		pageData := page.MustEval(jssdk.TPL_MIX_JS)
		pageHTML := fmt.Sprint(pageData)
		if cacher.IsEnable() && !config.DisableCache {
			err := cacher.Set(config.URL, pageHTML)
			if err != nil {
				logger.Instance.Warn("Unable to use cache")
			} else {
				if config.Expire > 0 {
					cacher.Expire(config.URL, fn.ExpireBySecond(config.Expire))
				} else {
					cacher.Expire(config.URL, fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION))
				}
			}
		}
		var emptyBody define.RemoteBodySanitized
		return ParseDataAndConfigBySSR(config, emptyBody, pageHTML)
	}

	injectCode := GetCSRInjectCode(config.File)
	pageData := page.MustEval(injectCode)
	pageHTML := fmt.Sprint(pageData)

	if cacher.IsEnable() && !config.DisableCache {
		err := cacher.Set(config.URL, pageHTML)
		if err != nil {
			logger.Instance.Warn("Unable to use cache")
		} else {
			if config.Expire > 0 {
				cacher.Expire(config.URL, fn.ExpireBySecond(config.Expire))
			} else {
				cacher.Expire(config.URL, fn.ExpireBySecond(define.IN_MEMORY_EXPIRATION))
			}
		}
	}

	code := define.ERROR_CODE_NULL
	status := define.ERROR_STATUS_NULL
	items := parseHTMLtoItems(pageHTML)
	now := time.Now()
	return define.MixupBodyParsed(code, status, now, items)
}

func ProxyPageByGoRod(url string, container string, proxyAddr string, cookies string) string {
	page := GetRodPageObject(container, proxyAddr, cookies)

	// TODO timeout set by config
	page.
		Timeout(fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT)).
		MustNavigate(url).
		MustWaitLoad().
		MustElement("body").
		CancelTimeout()

	pageData := page.MustEval(jssdk.TPL_MIX_JS)
	return fmt.Sprint(pageData)
}
