package rule

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestRulePathsAndReload(t *testing.T) {
	logger.Initialize()
	previous := define.RULES_DIRECTORY
	defer func() { define.RULES_DIRECTORY = previous; InitRules() }()
	root := filepath.Join(t.TempDir(), "nested", "rules")
	if err := os.MkdirAll(filepath.Join(root, "example"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "example", "feed.js")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "example", "README.md"), []byte("ignore"), 0600); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{root, relative, root + string(filepath.Separator)} {
		define.RULES_DIRECTORY = directory
		InitRules()
		got, ok := Lookup("example")
		if !ok || string(got.Body) != "first" || len(Snapshot()) != 1 {
			t.Fatalf("directory %q: %#v", directory, Snapshot())
		}
	}
	old := Snapshot()
	if err := os.WriteFile(file, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	InitRules()
	got, _ := Lookup("example")
	if string(got.Body) != "second" || string(old["example"].Body) != "first" {
		t.Fatal("reload did not publish a new snapshot")
	}
	delete(old, "example")
	if _, ok := Lookup("example"); !ok {
		t.Fatal("snapshot mutation affected active rules")
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				Lookup("example")
				Snapshot()
				InitRules()
			}
		}()
	}
	wg.Wait()
}
