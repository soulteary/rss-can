package rule

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/coalesce"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

type configEntry struct {
	value   define.JavaScriptConfig
	expires time.Time
}
type configStore struct {
	mu         sync.Mutex
	entries    map[string]configEntry
	generation uint64
	group      coalesce.Group[define.JavaScriptConfig]
}

var configurations configStore

// Caching is opt-in because rule configuration can depend on time or randomness.
func configCacheTTL() time.Duration {
	seconds, err := strconv.Atoi(os.Getenv("RSS_CONFIG_CACHE_TTL"))
	if err != nil || seconds <= 0 {
		return 0
	}
	if seconds > 3600 {
		seconds = 3600
	}
	return time.Duration(seconds) * time.Second
}

func (s *configStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]configEntry)
	s.generation++
}

func (s *configStore) load(ctx context.Context, rule define.RuleCache, ttl time.Duration, evaluate func(context.Context, define.RuleCache) (define.JavaScriptConfig, error), budgets ...time.Duration) (define.JavaScriptConfig, error) {
	if err := ctx.Err(); err != nil {
		return define.JavaScriptConfig{}, err
	}
	if ttl <= 0 {
		return evaluate(ctx, rule)
	}
	s.mu.Lock()
	generation := s.generation
	key := fmt.Sprintf("%d:%s:%x:%d", generation, rule.File, sha256.Sum256(rule.Body), ttl)
	entry, ok := s.entries[key]
	if ok && time.Now().Before(entry.expires) {
		s.mu.Unlock()
		telemetry.Default.Add("config_cache_hits_total", 1)
		return entry.value, nil
	}
	delete(s.entries, key)
	s.mu.Unlock()
	telemetry.Default.Add("config_cache_misses_total", 1)
	var timeout time.Duration
	if len(budgets) > 0 {
		timeout = budgets[0]
	} else {
		timeout = fn.ExpireBySecond(define.SERVER_TIMEOUT)
	}
	value, err, shared := s.group.Do(ctx, key, timeout, func(work context.Context) (define.JavaScriptConfig, error) {
		// A previous flight may have populated the cache between lookup and join.
		s.mu.Lock()
		entry, ok := s.entries[key]
		s.mu.Unlock()
		if ok && time.Now().Before(entry.expires) {
			return entry.value, nil
		}
		config, err := evaluate(work, rule)
		if err != nil {
			return config, err
		}
		if err := work.Err(); err != nil {
			return config, err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.generation != generation {
			return config, nil
		}
		if s.entries == nil {
			s.entries = make(map[string]configEntry)
		}
		if len(s.entries) >= 256 {
			var oldest string
			var expiry time.Time
			for k, e := range s.entries {
				if oldest == "" || e.expires.Before(expiry) {
					oldest, expiry = k, e.expires
				}
			}
			delete(s.entries, oldest)
		}
		s.entries[key] = configEntry{value: config, expires: time.Now().Add(ttl)}
		return config, nil
	})
	if shared {
		telemetry.Default.Add("config_shared_total", 1)
	}
	return value, err
}
