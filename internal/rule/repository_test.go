package rule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func writeRepositoryRule(t *testing.T, root, id, body string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "feed.js")
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}
func validRepositoryConfig(name string) define.JavaScriptConfig {
	return define.JavaScriptConfig{Name: name, Mode: "ssr", URL: "https://example.org", ListContainer: "article", Title: "h2"}
}
func repositoryEvaluator(_ context.Context, s define.RuleCache) (define.JavaScriptConfig, error) {
	if strings.Contains(string(s.Body), "invalid") {
		return define.JavaScriptConfig{}, errors.New("invalid configuration")
	}
	return validRepositoryConfig(string(s.Body)), nil
}
func TestRepositoryReloadIsAtomicAndRetainsLastGood(t *testing.T) {
	root := t.TempDir()
	file := writeRepositoryRule(t, root, "a", "first")
	repo := NewRepositoryWithEvaluator(root, repositoryEvaluator)
	if err := repo.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"invalid", "second"} {
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		err := repo.Reload(t.Context())
		got, _ := repo.Lookup("a")
		if body == "invalid" {
			if err == nil || string(got.Body) != "first" {
				t.Fatalf("failed reload published: %s %v", got.Body, err)
			}
		} else if err != nil || string(got.Body) != "second" {
			t.Fatalf("successful reload missing: %s %v", got.Body, err)
		}
	}
	old := repo.Snapshot()
	old["a"].Body[0] = '!'
	got, _ := repo.Lookup("a")
	got.Body[0] = '?'
	again, _ := repo.Lookup("a")
	if string(again.Body) != "second" {
		t.Fatal("published bytes mutated")
	}
	// Removing the directory is a scan failure, not an empty publication.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := repo.Reload(t.Context()); err == nil {
		t.Fatal("missing directory accepted")
	}
	if got, ok := repo.Lookup("a"); !ok || string(got.Body) != "second" {
		t.Fatal("scan failure lost old rules")
	}
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := repo.Reload(t.Context()); err == nil {
		t.Fatal("empty directory accepted")
	}
	if len(repo.Snapshot()) != 1 {
		t.Fatal("empty reload cleared old rules")
	}
}
func TestRepositoryRejectsReadFailureDuplicatesAndCancellation(t *testing.T) {
	root := t.TempDir()
	file := writeRepositoryRule(t, root, "a", "good")
	repo := NewRepositoryWithEvaluator(root, repositoryEvaluator)
	if err := repo.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(file), "other.js"), []byte("good"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := repo.Reload(t.Context()); err == nil {
		t.Fatal("duplicate ID accepted")
	}
	if err := os.Remove(filepath.Join(filepath.Dir(file), "other.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), file); err != nil {
		t.Fatal(err)
	}
	if err := repo.Reload(t.Context()); err == nil {
		t.Fatal("unreadable file accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := repo.Reload(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if len(repo.Snapshot()) != 1 {
		t.Fatal("failed candidate changed registry")
	}
}
func TestRepositoryValidationDoesNotFreezeDynamicConfig(t *testing.T) {
	t.Setenv("RSS_CONFIG_CACHE_TTL", "")
	root := t.TempDir()
	writeRepositoryRule(t, root, "a", "good")
	var evaluated atomic.Int32
	repo := NewRepositoryWithEvaluator(root, func(ctx context.Context, s define.RuleCache) (define.JavaScriptConfig, error) {
		evaluated.Add(1)
		return repositoryEvaluator(ctx, s)
	})
	if err := repo.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := repo.Lookup("a")
	for range 2 {
		if _, err := repo.Configure(t.Context(), snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if evaluated.Load() != 3 {
		t.Fatalf("validation unexpectedly cached dynamic value: %d", evaluated.Load())
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 10 {
				if err := repo.Reload(t.Context()); err != nil {
					t.Error(err)
				}
				if got := repo.Snapshot(); len(got) != 1 || string(got["a"].Body) != "good" {
					t.Error("partial snapshot observed")
				}
			}
		})
	}
	wg.Wait()
}
func TestRepositoryValidatesJavaScriptBeforePublish(t *testing.T) {
	logger.Initialize()
	root := t.TempDir()
	file := writeRepositoryRule(t, root, "a", `potted.SetScope("article").SetConfig({Name:"valid",Mode:"ssr",URL:"https://example.org",Title:"h2"});`)
	repo := NewRepository(root)
	if err := repo.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`throw new Error("broken");`, `potted.SetConfig({IdByRegexp:"["});`} {
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := repo.Reload(t.Context()); err == nil {
			t.Fatal("invalid rule published")
		}
	}
	got, _ := repo.Lookup("a")
	if strings.Contains(string(got.Body), "broken") {
		t.Fatal("invalid source replaced valid snapshot")
	}
}

func TestCompatibilityReloadRetainsSnapshotOnFailure(t *testing.T) {
	logger.Initialize()
	root := t.TempDir()
	file := writeRepositoryRule(t, root, "compat", `potted.SetScope("article").SetConfig({Name:"valid",URL:"https://example.org",Title:"h2"});`)
	previousDirectory := define.RULES_DIRECTORY
	rulesMu.Lock()
	previousRules := rulesCache
	rulesMu.Unlock()
	t.Cleanup(func() {
		define.RULES_DIRECTORY = previousDirectory
		rulesMu.Lock()
		rulesCache = previousRules
		rulesMu.Unlock()
		configurations.clear()
	})
	define.RULES_DIRECTORY = root
	if err := Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(`throw new Error("invalid")`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Reload(t.Context()); err == nil {
		t.Fatal("invalid compatibility reload accepted")
	}
	got, ok := Lookup("compat")
	if !ok || !strings.Contains(string(got.Body), "valid") {
		t.Fatal("failed reload changed compatibility registry")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Reload(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestRepositoryQueuedReloadCanBeCanceled(t *testing.T) {
	root := t.TempDir()
	writeRepositoryRule(t, root, "a", "good")
	entered, release := make(chan struct{}), make(chan struct{})
	repo := NewRepositoryWithEvaluator(root, func(ctx context.Context, s define.RuleCache) (define.JavaScriptConfig, error) {
		close(entered)
		<-release
		return repositoryEvaluator(ctx, s)
	})
	done := make(chan error, 1)
	go func() { done <- repo.Reload(t.Context()) }()
	<-entered
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := repo.Reload(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("queued reload: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCapturesCacheAndWorkBudgets(t *testing.T) {
	t.Setenv("RSS_CONFIG_CACHE_TTL", "30")
	previousTimeout := define.SERVER_TIMEOUT
	define.SERVER_TIMEOUT = 2
	t.Cleanup(func() { define.SERVER_TIMEOUT = previousTimeout })
	var calls int
	var budget time.Duration
	evaluate := func(ctx context.Context, _ define.RuleCache) (define.JavaScriptConfig, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("shared evaluation has no deadline")
		}
		budget = time.Until(deadline)
		return validRepositoryConfig("captured"), nil
	}
	repo := NewRepositoryWithEvaluator("unused", evaluate)
	t.Setenv("RSS_CONFIG_CACHE_TTL", "0")
	define.SERVER_TIMEOUT = 100
	snapshot := define.RuleCache{Body: []byte("captured")}
	for range 2 {
		if _, err := repo.Configure(t.Context(), snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("captured TTL changed with environment: %d calls", calls)
	}
	if budget <= 0 || budget > 2*time.Second {
		t.Fatalf("captured timeout changed with global: %v", budget)
	}
	other := NewRepositoryWithOptions("unused", evaluate, 0, time.Second)
	// Disabling configuration caching keeps evaluations in caller context.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	for range 2 {
		if _, err := other.Configure(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatalf("independent disabled cache reused config: %d", calls)
	}
}
