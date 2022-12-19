package rule

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/javascript"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func getDirRuleFiles(baseDir string) (ruleFiles []string) {
	rules, err := os.ReadDir(baseDir)
	if err != nil {
		logger.Instance.Errorf("Scan rule rules not complete: %v", err)
		return nil
	}

	for _, file := range rules {
		ruleFiles = append(ruleFiles, filepath.Join(baseDir, file.Name()))
	}

	if len(ruleFiles) == 0 {
		logger.Instance.Warnln("Scanning the rules directory completed, but no configuration files were found.")
	}
	return ruleFiles
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

func LoadRules(ruleDir string) []string {
	ruleFiles := getDirRuleFiles(ruleDir)

	var rules []string
	for _, item := range ruleFiles {
		if isDir(item) == 1 {
			subFiles := getDirRuleFiles(item)
			for _, sItem := range subFiles {
				if isDir(sItem) == 0 {
					rules = append(rules, sItem)
				}
			}
		}
	}

	logger.Instance.Infof("Load rules, count: %d", len(rules))
	return rules
}

func generateSDKsByRuleFile(file string) (sdk string, app string, err error) {
	jsRule, err := os.ReadFile(file)
	if err != nil {
		return sdk, app, err
	}

	sdk = fmt.Sprintf("%s\n%s\n", jssdk.SSR_SHIM, jssdk.SDK)
	app = fmt.Sprintf("var potted = new POTTED();\n%s\n%s", jsRule, "JSON.stringify(potted.GetConfig());")
	return sdk, app, nil
}

func GenerateConfigByRule(rule string) (config define.JavaScriptConfig, err error) {
	base, app, err := generateSDKsByRuleFile(rule)
	if err != nil {
		logger.Instance.Errorf("Read rule file failed: %v", err)
		return config, err
	}

	jsConfig, err := javascript.RunCode(base, app)
	if err != nil {
		logger.Instance.Errorf("Executing rule file failed: %v", err)
		return config, err
	}

	config, err = ParseConfigFromJSON(jsConfig, rule)
	if err != nil {
		logger.Instance.Errorf("Parsing rule file failed: %v", err)
	}
	return config, err
}
