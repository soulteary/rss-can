package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/soulteary/RSS-Can/internal/define"
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
		browser = rod.New().ControlURL(l.MustLaunch()).Trace(true).SlowMotion(2 * time.Second)
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

	return page
}

const INJECT_CODE_MIX_PARSER = `()=> document.documentElement.innerHTML`
const INJECT_CODE_CSR_PARSER = `
()=> (function(window){
%s
var potted = new POTTED();
%s;
potted.GetData();
return potted.value;
})(window)`

func GetCSRInjectCode(file string) string {
	jsCSR, err := os.ReadFile("./internal/jssdk/jquery.min.js")
	if err != nil {
		fmt.Println(err)
		return ""
	}

	jsSDK, _ := os.ReadFile("./internal/jssdk/sdk.js")
	if err != nil {
		fmt.Println(err)
		return ""
	}

	jsRule, err := os.ReadFile(file)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	jsApp := fmt.Sprintf("%s\n%s\n", jsCSR, jsSDK)
	return fmt.Sprintf(INJECT_CODE_CSR_PARSER, jsApp, string(jsRule))
}

func ParsePageByGoRod(config define.JavaScriptConfig, container string, proxyAddr string, useMixParser bool) (result define.BodyParsed) {
	page := GetRodPageObject(container, proxyAddr)

	page.
		Timeout(5 * time.Second).
		MustNavigate(config.URL).
		MustWaitLoad().
		MustElement(config.ListContainer).
		CancelTimeout()

	if useMixParser {
		pageHTML := page.MustEval(INJECT_CODE_MIX_PARSER)
		var emptyBody define.RemoteBodySanitized
		return ParseDataAndConfigBySSR(config, emptyBody, fmt.Sprint(pageHTML))
	}

	injectCode := GetCSRInjectCode(config.File)
	pageData := page.MustEval(injectCode)
	var items []define.InfoItem
	err := json.Unmarshal([]byte(fmt.Sprint(pageData)), &items)
	if err != nil {
		fmt.Println(err)
		return result
	}

	code := define.ERROR_CODE_NULL
	status := define.ERROR_STATUS_NULL
	now := time.Now()
	return define.MixupBodyParsed(code, status, now, items)
}
