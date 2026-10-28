package rule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

// ConfigEvaluator evaluates a captured script without reading the filesystem.
type ConfigEvaluator func(context.Context, define.RuleCache) (define.JavaScriptConfig, error)

// Repository publishes complete validated snapshots. A failed reload never
// replaces the last good snapshot. Empty rule sets are rejected deliberately.
type Repository struct {
	directory      string
	reloadMu       chan struct{}
	mu             sync.RWMutex
	rules          map[string]define.RuleCache
	configurations configStore
	evaluate       ConfigEvaluator
	cacheTTL       time.Duration
	timeout        time.Duration
}

func NewRepository(directory string) *Repository {
	return NewRepositoryWithEvaluator(directory, evaluateConfig)
}
func NewRepositoryWithEvaluator(directory string, evaluate ConfigEvaluator) *Repository {
	return NewRepositoryWithOptions(directory, evaluate, configCacheTTL(), fn.ExpireBySecond(define.SERVER_TIMEOUT))
}

// NewRepositoryWithOptions captures cache and shared-work budgets for one app.
func NewRepositoryWithOptions(directory string, evaluate ConfigEvaluator, ttl, timeout time.Duration) *Repository {
	if timeout <= 0 {
		panic("rule: configuration timeout must be positive")
	}
	if evaluate == nil {
		panic("rule: missing configuration evaluator")
	}
	return &Repository{directory: directory, reloadMu: make(chan struct{}, 1), rules: make(map[string]define.RuleCache), evaluate: evaluate, cacheTTL: ttl, timeout: timeout}
}
func cloneRule(item define.RuleCache) define.RuleCache {
	item.Body = append([]byte(nil), item.Body...)
	return item
}
func (r *Repository) Lookup(id string) (define.RuleCache, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.rules[id]
	return cloneRule(item), ok
}
func (r *Repository) Snapshot() map[string]define.RuleCache {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]define.RuleCache, len(r.rules))
	for id, item := range r.rules {
		result[id] = cloneRule(item)
	}
	return result
}
func (r *Repository) Configure(ctx context.Context, snapshot define.RuleCache) (define.JavaScriptConfig, error) {
	return r.configurations.load(ctx, snapshot, r.cacheTTL, r.evaluate, r.timeout)
}

func (r *Repository) Reload(ctx context.Context) error {
	select {
	case r.reloadMu <- struct{}{}:
		defer func() { <-r.reloadMu }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	candidate, err := loadCandidate(ctx, r.directory, r.evaluate)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	r.rules = candidate
	r.configurations.clear()
	return nil
}

// readRuleFiles propagates every filesystem error. Unlike the historical scan
// helper, a missing directory cannot be mistaken for an intentionally empty one.
func readRuleFiles(ctx context.Context, directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("scan rule directory: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		subdir := filepath.Join(directory, entry.Name())
		children, err := os.ReadDir(subdir)
		if err != nil {
			return nil, fmt.Errorf("scan rule group %s: %w", entry.Name(), err)
		}
		for _, child := range children {
			if child.IsDir() || !strings.EqualFold(filepath.Ext(child.Name()), ".js") {
				continue
			}
			files = append(files, filepath.Join(subdir, child.Name()))
		}
	}
	return files, nil
}
func loadCandidate(ctx context.Context, directory string, evaluate ConfigEvaluator) (map[string]define.RuleCache, error) {
	files, err := readRuleFiles(ctx, directory)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("rule reload rejected: no JavaScript rules in %s", directory)
	}
	candidate := make(map[string]define.RuleCache, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id := filepath.Base(filepath.Dir(file))
		if _, exists := candidate[id]; exists {
			return nil, fmt.Errorf("rule reload rejected: duplicate rule ID %s", id)
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read rule %s: %w", file, err)
		}
		snapshot := define.RuleCache{Body: body, Time: time.Now(), Sign: fn.GetFileSHA1(body), File: file}
		if _, err := evaluate(ctx, snapshot); err != nil {
			return nil, fmt.Errorf("validate rule %s: %w", id, err)
		}
		candidate[id] = snapshot
	}
	return candidate, nil
}

var reloadMu = make(chan struct{}, 1)

// Reload validates the compatibility registry transactionally. Production apps
// can own an independent Repository instead of this package-level registry.
func Reload(ctx context.Context) error {
	select {
	case reloadMu <- struct{}{}:
		defer func() { <-reloadMu }()
	case <-ctx.Done():
		return ctx.Err()
	}
	candidate, err := loadCandidate(ctx, define.RULES_DIRECTORY, evaluateConfig)
	if err != nil {
		return err
	}
	rulesMu.Lock()
	defer rulesMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	rulesCache = candidate
	configurations.clear()
	return nil
}

// EvaluateConfigContext evaluates a captured source without configuration caching.
// Applications may wrap its context to select their owned JavaScript runtime.
func EvaluateConfigContext(ctx context.Context, snapshot define.RuleCache) (define.JavaScriptConfig, error) {
	return evaluateConfig(ctx, snapshot)
}
