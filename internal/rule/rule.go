package rule

import (
	"fmt"
	"os"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func LoadRules(ruleDir string) []string {
	ruleFiles := fn.ScanDirFiles(ruleDir)

	var rules []string
	for _, item := range ruleFiles {
		if fn.IsDir(item) {
			subFiles := fn.ScanDirFiles(item)
			for _, sItem := range subFiles {
				if fn.IsFile(sItem) {
					rules = append(rules, sItem)
				}
			}
		}
	}

	if len(rules) == 0 {
		logger.Instance.Warnln("Scanning the rules directory completed, but no configuration files were found.")
	} else {
		logger.Instance.Infof("Load rules, count: %d", len(rules))
	}

	return rules
}

func generateSDKsByRuleFile(file string) (app string, err error) {
	jsRule, err := os.ReadFile(file)
	if err != nil {
		return app, err
	}

	app = fmt.Sprintf("var potted = new POTTED();\n%s\n%s", jsRule, "JSON.stringify(potted.GetConfig());")
	return app, nil
}

func GenerateConfigByRule(rule string) (config define.JavaScriptConfig, err error) {
	app, err := generateSDKsByRuleFile(rule)
	if err != nil {
		logger.Instance.Errorf("Read rule file failed: %v", err)
		return config, err
	}

	jsConfig, err := jssdk.RunCode(jssdk.TPL_SSR_JS, app)
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
