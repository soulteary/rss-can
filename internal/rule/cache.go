package rule

import (
	"strings"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

var RulesCache map[string]define.RuleCache

func makeMap(list []string) map[string]define.RuleCache {
	result := make(map[string]define.RuleCache)
	for _, file := range list {
		body := fn.GetFileContent(file)
		sha1 := fn.GetFileSHA1(body)
		item := define.RuleCache{Body: body, Time: time.Now(), Sign: sha1, File: file}
		result[strings.Split(file, "/")[1]] = item
	}
	return result
}

func InitRules() {
	rules := LoadRules(define.RULES_DIRECTORY)
	newCache := makeMap(rules)

	RulesCache = newCache
}
