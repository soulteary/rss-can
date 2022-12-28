package rule

import (
	"os"
	"strings"

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

	return jssdk.GenerateGetConfigWithRule(jsRule), nil
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

	config.Name = strings.TrimSpace(config.Name)
	config.URL = strings.TrimSpace(config.URL)
	config.Mode = strings.TrimSpace(config.Mode)
	config.File = strings.TrimSpace(config.File)
	config.Charset = strings.TrimSpace(config.Charset)
	config.Headless = strings.TrimSpace(config.Headless)
	config.Proxy = strings.TrimSpace(config.Proxy)

	config.IdByRegexp = strings.TrimSpace(config.IdByRegexp)
	config.ListContainer = strings.TrimSpace(config.ListContainer)
	config.Title = strings.TrimSpace(config.Title)
	config.Author = strings.TrimSpace(config.Author)
	config.Link = strings.TrimSpace(config.Link)
	config.DateTime = strings.TrimSpace(config.DateTime)
	config.Description = strings.TrimSpace(config.Description)
	config.Pager = strings.TrimSpace(config.Pager)

	config.DateTimeHook.Action = strings.TrimSpace(config.DateTimeHook.Action)
	config.DateTimeHook.Object = strings.TrimSpace(config.DateTimeHook.Object)
	config.DateTimeHook.URL = strings.TrimSpace(config.DateTimeHook.URL)

	config.CategoryHook.Action = strings.TrimSpace(config.CategoryHook.Action)
	config.CategoryHook.Object = strings.TrimSpace(config.CategoryHook.Object)
	config.CategoryHook.URL = strings.TrimSpace(config.CategoryHook.URL)

	config.DescriptionHook.Action = strings.TrimSpace(config.DescriptionHook.Action)
	config.DescriptionHook.Object = strings.TrimSpace(config.DescriptionHook.Object)
	config.DescriptionHook.URL = strings.TrimSpace(config.DescriptionHook.URL)

	config.ContentHook.Action = strings.TrimSpace(config.ContentHook.Action)
	config.ContentHook.Object = strings.TrimSpace(config.ContentHook.Object)
	config.ContentHook.URL = strings.TrimSpace(config.ContentHook.URL)

	return config, err
}
