package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/network"
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
	if rule.Body != nil {
		config.RuleSource = define.NewRuleSource(rule.Body)
	}
	if _, err := config.CompileIDRegexp(); err != nil {
		return config, err
	}
	if config.PagerLimit > limits.MaxPages {
		return config, fmt.Errorf("PagerLimit exceeds maximum %d", limits.MaxPages)
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
	proxy, err := network.NormalizeProxy(network.OptionsForConfigContext(ctx, config).Proxy)
	if err != nil {
		return define.MixupBodyParsed(define.ERROR_CODE_INIT_NETWORK_FAILED, err.Error(), time.Now(), nil)
	}
	switch strings.ToLower(config.Mode) {
	case define.PARSE_MODE_SSR:
		return parser.GetDataAndConfigBySSRContext(ctx, config)
	case define.PARSE_MODE_CSR, define.PARSE_MODE_MIX:
		container := define.HEADLESS_SERVER
		if fn.IsVaildHeadlessAddr(config.Headless) {
			container = config.Headless
		}
		return parser.ParsePageByBrowserContext(ctx, config, container, proxy, strings.EqualFold(config.Mode, define.PARSE_MODE_MIX))
	}

	// TODO handle remote mode(api) ...
	return result
}
