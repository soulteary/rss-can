package csr

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

func ParsePageByGoRod(config define.JavaScriptConfig, container string, proxyAddr string) (result define.BodyParsed) {
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

	page.MustEvalOnNewDocument(`window.alert = () => {};window.prompt = () => {}`)

	page.
		Timeout(5 * time.Second).
		MustNavigate(config.URL).
		MustWaitLoad().
		MustElement(config.ListContainer).
		CancelTimeout()

	jsCSR, _ := os.ReadFile("./internal/jssdk/jquery.min.js")
	jsSDK, _ := os.ReadFile("./internal/jssdk/sdk.js")
	jsApp := fmt.Sprintf("%s\n%s\n", jsCSR, jsSDK)

	jsRule, err := os.ReadFile(config.File)
	if err != nil {
		fmt.Println(err)
		return result
	}
	inject := page.MustEval(fmt.Sprintf(`
()=> (function(window){
%s
var potted = new POTTED();
%s;
potted.GetData();
return potted.value;
})(window)`, string(jsApp), string(jsRule)))

	now := time.Now()
	var items []define.InfoItem
	json.Unmarshal([]byte(fmt.Sprint(inject)), &items)
	if err != nil {
		fmt.Println(err)
		return result
	}
	code := define.ERROR_CODE_NULL
	status := define.ERROR_STATUS_NULL
	return define.MixupBodyParsed(code, status, now, items)
}
