//go:build darwin || linux

package rule

import (
	"context"
	"fmt"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/rulefile"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositoryRejectsSpecialRulesWithoutLosingMetadata(t *testing.T) {
	root := t.TempDir()
	file := writeRepositoryRule(t, root, "a", "good")
	repo := NewRepositoryWithEvaluator(root, repositoryEvaluator)
	if err := repo.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	initial := repo.SnapshotMetadata()["a"]
	if initial.Name != "good" || initial.File != file || initial.Sign == "" {
		t.Fatalf("metadata: %+v", initial)
	}
	copied := repo.SnapshotMetadata()
	copied["a"] = Metadata{Name: "changed"}
	if repo.SnapshotMetadata()["a"] != initial {
		t.Fatal("metadata reader changed published state")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(file, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- repo.Reload(context.Background()) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("reload blocked opening FIFO")
	}
	if repo.SnapshotMetadata()["a"] != initial {
		t.Fatal("failed reload replaced metadata")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "real.js")
	if err := os.WriteFile(target, []byte("good"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, file); err != nil {
		t.Fatal(err)
	}
	if err := repo.Reload(t.Context()); err == nil {
		t.Fatal("symlink rule accepted")
	}
	if err := os.RemoveAll(filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(target), filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Reload(t.Context()); err == nil {
		t.Fatal("symlink rule group accepted")
	}
	if repo.SnapshotMetadata()["a"] != initial {
		t.Fatal("failed symlink reload replaced metadata")
	}
}

func TestRepositoryRejectsFileCountTotalBytesAndDisplayName(t *testing.T) {
	evaluator := func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
		return define.JavaScriptConfig{Name: "bounded"}, nil
	}
	for _, which := range []string{"file", "count", "total", "name"} {
		t.Run(which, func(t *testing.T) {
			root := t.TempDir()
			file := writeRepositoryRule(t, root, "a", "good")
			repo := NewRepositoryWithEvaluator(root, evaluator)
			if err := repo.Reload(t.Context()); err != nil {
				t.Fatal(err)
			}
			initial := repo.SnapshotMetadata()["a"]
			switch which {
			case "file":
				if err := os.WriteFile(file, []byte(strings.Repeat("x", rulefile.MaxBytes+1)), 0600); err != nil {
					t.Fatal(err)
				}
			case "count":
				for i := 0; i < rulefile.MaxFiles; i++ {
					writeRepositoryRule(t, root, fmt.Sprintf("extra%03d", i), "good")
				}
			case "total":
				for i := 0; i < 17; i++ {
					writeRepositoryRule(t, root, fmt.Sprintf("large%03d", i), strings.Repeat("x", rulefile.MaxBytes))
				}
			case "name":
				repo.evaluate = func(context.Context, define.RuleCache) (define.JavaScriptConfig, error) {
					return define.JavaScriptConfig{Name: strings.Repeat("x", MaxMetadataNameBytes+1)}, nil
				}
			}
			if err := repo.Reload(t.Context()); err == nil {
				t.Fatalf("%s limit accepted", which)
			}
			if len(repo.Snapshot()) != 1 || repo.SnapshotMetadata()["a"] != initial {
				t.Fatal("rejected reload lost valid snapshot")
			}
		})
	}
}
