package parser

import (
	"encoding/json"
	"fmt"
	"os"
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

func GetRodPageObject(container string, proxyAddr string) *rod.Page {
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
	page.MustEvalOnNewDocument(`window.alert = () => {};window.prompt = () => {}`)

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

	return page
}

const INJECT_CODE_MIX_PARSER = `()=> document.documentElement.innerHTML`

func GetCSRInjectCode(file string) string {
	jsRule, err := os.ReadFile(file)
	if err != nil {
		logger.Instance.Errorf("Open rule failed %v", err)
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

	page := GetRodPageObject(container, proxyAddr)

	// TODO timeout set by config
	// TODO support pager config
	page.
		Timeout(fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT)).
		MustNavigate(config.URL).
		MustWaitLoad().
		MustElement(config.ListContainer).
		CancelTimeout()

	if useMixParser {
		pageData := page.MustEval(INJECT_CODE_MIX_PARSER)
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

func ProxyPageByGoRod(url string, container string, proxyAddr string) string {
	page := GetRodPageObject(container, proxyAddr)

	// TODO timeout set by config
	page.
		Timeout(fn.ExpireBySecond(define.HEADLESS_EXCUTE_TIMEOUT)).
		MustNavigate(url).
		MustWaitLoad().
		MustElement("body").
		CancelTimeout()

	pageData := page.MustEval("()=>document.documentElement.innerHTML")
	return fmt.Sprint(pageData)
}
