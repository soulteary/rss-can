package parser

import (
	"encoding/json"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
)

func JSONStringify(r interface{}) (string, error) {
	out, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

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
	if !(modeInRuls == "ssr" || modeInRuls == "csr") {
		config.Mode = "ssr"
	}
	return config
}
