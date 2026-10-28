package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/parser"
)

func ParseConfigFromJSON(str string, rule define.RuleCache) (define.JavaScriptConfig, error) {
	var config define.JavaScriptConfig
	err := json.Unmarshal([]byte(str), &config)
	if err != nil {
		return config, err
	}

	if rule.File != "" {
		config.File = rule.File
	}
	if _, err := config.CompileIDRegexp(); err != nil {
		return config, err
	}
	if config.PagerLimit < 0 {
		return config, fmt.Errorf("PagerLimit must not be negative")
	}

	modeInRule := strings.ToLower(config.Mode)
	if !fn.IsStrInArray([]string{define.PARSE_MODE_SSR, define.PARSE_MODE_CSR, define.PARSE_MODE_MIX}, modeInRule) {
		config.Mode = define.DEFAULT_PARSE_MODE
	}
	return config, nil
}

func GetWebsiteDataWithConfig(config define.JavaScriptConfig) (result define.BodyParsed) {
	return GetWebsiteDataWithConfigContext(context.Background(), config)
}

func GetWebsiteDataWithConfigContext(ctx context.Context, config define.JavaScriptConfig) (result define.BodyParsed) {
	switch strings.ToLower(config.Mode) {
	case define.PARSE_MODE_SSR:
		return parser.GetDataAndConfigBySSRContext(ctx, config)
	case define.PARSE_MODE_CSR:
		container := define.HEADLESS_SERVER
		if fn.IsVaildHeadlessAddr(config.Headless) {
			container = config.Headless
		}
		proxy := define.PROXY_SERVER
		if fn.IsVaildProxyAddr(config.Proxy) {
			proxy = config.Proxy
		}
		return parser.ParsePageByBrowserContext(ctx, config, container, proxy, false)
	case define.PARSE_MODE_MIX:
		container := define.HEADLESS_SERVER
		if fn.IsVaildHeadlessAddr(config.Headless) {
			container = config.Headless
		}
		proxy := define.PROXY_SERVER
		if fn.IsVaildProxyAddr(config.Proxy) {
			proxy = config.Proxy
		}
		return parser.ParsePageByBrowserContext(ctx, config, container, proxy, true)
	}

	// TODO handle remote mode(api) ...
	return result
}
