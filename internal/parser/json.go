package parser

import (
	"encoding/json"

	"github.com/soulteary/RSS-Can/internal/define"
)

func JSONStringify(r interface{}) (string, error) {
	out, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func ParseConfigFromJSON(str string) (define.JavaScriptConfig, error) {
	var config define.JavaScriptConfig
	err := json.Unmarshal([]byte(str), &config)
	if err != nil {
		return config, err
	}
	return config, nil
}
