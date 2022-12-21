package rule

import (
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
)

var RulesCache map[string]string

func makeMap(list []string) map[string]string {
	result := make(map[string]string)
	for _, s := range list {
		result[strings.Split(s, "/")[1]] = s
	}
	return result
}

func InitRules() {
	rules := LoadRules(define.RULES_DIRECTORY)
	newCache := makeMap(rules)

	RulesCache = newCache
}

func GetRuleByName(name string) (string, bool) {
	file, ok := RulesCache[name]
	if !ok {
		return "", false
	}
	return file, true
}
