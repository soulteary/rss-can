package rule

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
)

var rulesMu sync.RWMutex
var rulesCache = map[string]define.RuleCache{}

func Lookup(id string) (define.RuleCache, bool) {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	item, ok := rulesCache[id]
	item.Body = append([]byte(nil), item.Body...)
	return item, ok
}

// Snapshot returns a consistent copy so readers do not race with a reload.
func Snapshot() map[string]define.RuleCache {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	result := make(map[string]define.RuleCache, len(rulesCache))
	for id, item := range rulesCache {
		item.Body = append([]byte(nil), item.Body...)
		result[id] = item
	}
	return result
}

func makeMap(list []string) map[string]define.RuleCache {
	result := make(map[string]define.RuleCache)
	for _, file := range list {
		id := filepath.Base(filepath.Dir(file))
		if _, exists := result[id]; exists {
			logger.Instance.Warnf("Ignoring duplicate rule ID %s: %s", id, file)
			continue
		}
		body := fn.GetFileContent(file)
		if body == nil {
			continue
		}
		result[id] = define.RuleCache{Body: body, Time: time.Now(), Sign: fn.GetFileSHA1(body), File: file}
	}
	return result
}

func InitRules() {
	newCache := makeMap(LoadRules(define.RULES_DIRECTORY))
	rulesMu.Lock()
	rulesCache = newCache
	configurations.clear()
	rulesMu.Unlock()
}
