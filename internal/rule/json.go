package rule

import (
	"encoding/json"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/parser"
)

func ParseConfigFromJSON(str string, ruleFile string) (define.JavaScriptConfig, error) {
	var config define.JavaScriptConfig
	err := json.Unmarshal([]byte(str), &config)
	if err != nil {
		return config, err
	}
	if ruleFile != "" {
		config.File = ruleFile
	}
	return ApplyDefaults(config), nil
}

// TODO: warning when value fixed by default
func ApplyDefaults(config define.JavaScriptConfig) define.JavaScriptConfig {
	modeInRuls := strings.ToLower(config.Mode)
	if !(modeInRuls == "ssr" || modeInRuls == "csr" || modeInRuls == "mix") {
		config.Mode = "ssr"
	}
	return config
}

func GetWebsiteDataWithConfig(config define.JavaScriptConfig) (result define.BodyParsed) {
	switch strings.ToLower(config.Mode) {
	case "ssr":
		return parser.GetDataAndConfigBySSR(config)
	case "csr":
		const container = "127.0.0.1:9222"
		const proxy = ""
		return parser.GetDataAndConfigByCSR(config, container, proxy)
	case "mix":
		const container = "127.0.0.1:9222"
		const proxy = ""
		return parser.GetDataAndConfigByMix(config, container, proxy)
	}

	// TODO handle remote ...
	return result
}
