package rule

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/javascript"
)

func scanFiles(dir string) (result []string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, item := range files {
		result = append(result, filepath.Join(dir, item.Name()))
	}
	return result
}

func isDir(input string) int {
	target, err := os.Stat(input)
	if err != nil {
		return -1
	}
	if target.IsDir() {
		return 1
	}
	return 0
}

func LoadRules() []string {
	const configDir = "./rules"
	files := scanFiles(configDir)

	var rules []string
	for _, file := range files {
		if isDir(file) == 1 {
			subFiles := scanFiles(file)
			for _, sItem := range subFiles {
				if isDir(sItem) == 0 {
					rules = append(rules, sItem)
				}
			}
		}
	}
	return rules
}

func GetSDKs(file string) (sdk string, app string, err error) {
	jsSSR, err := os.ReadFile("./internal/jssdk/ssr.js")
	if err != nil {
		return sdk, app, errors.New("SSR JavaScript SDK(shim) load failed")
	}

	jsSDK, err := os.ReadFile("./internal/jssdk/sdk.js")
	if err != nil {
		return sdk, app, errors.New("SSR JavaScript SDK(app) load failed")
	}

	jsRule, err := os.ReadFile(file)
	if err != nil {
		return sdk, app, errors.New("SSR JavaScript Rule load failed")
	}

	sdk = fmt.Sprintf("%s\n%s\n", jsSSR, jsSDK)
	app = fmt.Sprintf("var potted = new POTTED();\n%s\n%s", jsRule, "JSON.stringify(potted.GetConfig());")
	return sdk, app, nil
}

func GenerateConfigByRule(rule string) (config define.JavaScriptConfig, err error) {
	base, app, err := GetSDKs(rule)
	if err != nil {
		return config, err
	}

	jsConfig, err := javascript.RunCode(base, app)
	if err != nil {
		return config, err
	}

	config, err = ParseConfigFromJSON(jsConfig, rule)
	return config, err
}
