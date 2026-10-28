package rule

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/rulefile"
)

// ConfigEvaluator evaluates a captured script without reading the filesystem.
const MaxMetadataNameBytes = 4096

type Metadata struct{ Name, File, Sign string }

type ConfigEvaluator func(context.Context, define.RuleCache) (define.JavaScriptConfig, error)

// Repository publishes complete validated snapshots. A failed reload never
// replaces the last good snapshot. Empty rule sets are rejected deliberately.
type Repository struct {
	directory      string
	reloadMu       chan struct{}
	mu             sync.RWMutex
	rules          map[string]define.RuleCache
	metadata       map[string]Metadata
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

// SnapshotMetadata exposes display-only values captured during successful
// validation. It never evaluates JavaScript or copies rule source bytes.
func (r *Repository) SnapshotMetadata() map[string]Metadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Metadata, len(r.metadata))
	for id, item := range r.metadata {
		result[id] = item
	}
	return result
}
func (r *Repository) Configure(ctx context.Context, snapshot define.RuleCache) (define.JavaScriptConfig, error) {
	return r.configurations.load(ctx, snapshot, r.cacheTTL, func(work context.Context, snapshot define.RuleCache) (define.JavaScriptConfig, error) {
		config, err := r.evaluate(work, snapshot)
		if err == nil {
			err = ValidateConfig(config)
		}
		return config, err
	}, r.timeout)
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
	candidate, metadata, err := loadCandidate(ctx, r.directory, r.evaluate)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	r.rules = candidate
	r.metadata = metadata
	r.configurations.clear()
	return nil
}

// scanRuleFiles reads bounded batches and rejects symbolic-link groups and
// non-regular .js entries rather than silently omitting part of a candidate.
func scanRuleFiles(ctx context.Context, root *rulefile.Directory) ([]string, error) {
	var files []string
	groups := 0
	for {
		entries, err := root.ReadDir(64)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("scan rule directory: %w", err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("rule group must not be a symbolic link: %s", entry.Name())
			}
			if !entry.IsDir() {
				continue
			}
			groups++
			if groups > rulefile.MaxFiles {
				return nil, fmt.Errorf("rule groups exceed maximum %d", rulefile.MaxFiles)
			}
			group, err := root.OpenDirectory(entry.Name())
			if err != nil {
				return nil, fmt.Errorf("open rule group %s: %w", entry.Name(), err)
			}
			children, err := scanRuleGroup(ctx, group, entry.Name(), rulefile.MaxFiles-len(files))
			group.Close()
			if err != nil {
				return nil, err
			}
			files = append(files, children...)
		}
		if err == io.EOF {
			break
		}
	}
	return files, nil
}
func scanRuleGroup(ctx context.Context, group *rulefile.Directory, name string, remaining int) ([]string, error) {
	var files []string
	for {
		children, err := group.ReadDir(64)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("scan rule group %s: %w", name, err)
		}
		for _, child := range children {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !strings.EqualFold(filepath.Ext(child.Name()), ".js") {
				continue
			}
			if !child.Type().IsRegular() {
				return nil, fmt.Errorf("rule must be a regular file: %s", filepath.Join(name, child.Name()))
			}
			if len(files) >= remaining {
				return nil, fmt.Errorf("rule count exceeds maximum %d", rulefile.MaxFiles)
			}
			files = append(files, filepath.Join(name, child.Name()))
		}
		if err == io.EOF {
			break
		}
	}
	return files, nil
}
func loadCandidate(ctx context.Context, directory string, evaluate ConfigEvaluator) (map[string]define.RuleCache, map[string]Metadata, error) {
	root, err := rulefile.OpenDirectory(directory)
	if err != nil {
		return nil, nil, fmt.Errorf("scan rule directory: %w", err)
	}
	defer root.Close()
	files, err := scanRuleFiles(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("rule reload rejected: no JavaScript rules in %s", directory)
	}
	candidate := make(map[string]define.RuleCache, len(files))
	metadata := make(map[string]Metadata, len(files))
	total := 0
	for _, relative := range files {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		id := filepath.Dir(relative)
		if _, exists := candidate[id]; exists {
			return nil, nil, fmt.Errorf("rule reload rejected: duplicate rule ID %s", id)
		}
		group, err := root.OpenDirectory(id)
		if err != nil {
			return nil, nil, fmt.Errorf("open rule group %s: %w", id, err)
		}
		body, err := group.ReadFile(ctx, filepath.Base(relative), min(rulefile.MaxBytes, rulefile.MaxTotalBytes-total))
		group.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("read rule %s: %w", relative, err)
		}
		total += len(body)
		file := filepath.Join(directory, relative)
		snapshot := define.RuleCache{Body: body, Time: time.Now(), Sign: fn.GetFileSHA1(body), File: file}
		config, err := evaluate(ctx, snapshot)
		if err != nil {
			return nil, nil, fmt.Errorf("validate rule %s: %w", id, err)
		}
		if err := ValidateConfig(config); err != nil {
			return nil, nil, fmt.Errorf("validate rule %s: %w", id, err)
		}
		if len(config.Name) > MaxMetadataNameBytes {
			return nil, nil, fmt.Errorf("rule %s display name exceeds %d bytes", id, MaxMetadataNameBytes)
		}
		candidate[id] = snapshot
		metadata[id] = Metadata{Name: config.Name, File: file, Sign: snapshot.Sign}
	}
	return candidate, metadata, nil
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
	candidate, _, err := loadCandidate(ctx, define.RULES_DIRECTORY, evaluateConfig)
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
