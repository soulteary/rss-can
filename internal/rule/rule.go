package rule

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/javascript"
	"github.com/soulteary/RSS-Can/internal/parser"
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

func GenerateConfigByRule(rule string) (config define.JavaScriptConfig, err error) {
	jsSSR, _ := os.ReadFile("./internal/jssdk/ssr.js")
	jsSDK, _ := os.ReadFile("./internal/jssdk/sdk.js")
	base := fmt.Sprintf("%s\n%s\n", jsSSR, jsSDK)

	jsApp, _ := os.ReadFile(rule)
	inject := fmt.Sprintf("var potted = new POTTED();\n%s\n%s", jsApp, "JSON.stringify(potted.GetConfig());")

	jsConfig, err := javascript.RunCode(base, inject)
	if err != nil {
		return config, err
	}

	config, err = parser.ParseConfigFromJSON(jsConfig)
	return config, err
}
