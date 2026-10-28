package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestBrowserRuleSnapshotSurvivesFileChanges(t *testing.T) {
	file := filepath.Join(t.TempDir(), "rule.js")
	original := []byte(`potted.SetConfig({Name:"original"});`)
	config := define.JavaScriptConfig{File: file, RuleSource: define.NewRuleSource(original)}
	key := browserCacheKey(config, false)
	original[0] = '!'
	if err := os.WriteFile(file, []byte(`throw new Error("replacement");`), 0600); err != nil {
		t.Fatal(err)
	}
	captured, err := captureBrowserRule(config)
	if err != nil {
		t.Fatal(err)
	}
	if browserCacheKey(captured, false) != key || !strings.Contains(csrInjectCode(captured), `Name:"original"`) || strings.Contains(csrInjectCode(captured), "replacement") {
		t.Fatal("file change altered published source or cache identity")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	captured, err = captureBrowserRule(config)
	if err != nil || browserCacheKey(captured, true) != browserCacheKey(config, true) {
		t.Fatalf("snapshot depends on deleted file: %v", err)
	}
	changed := config
	changed.RuleSource = define.NewRuleSource([]byte("replacement"))
	if browserCacheKey(changed, false) == key {
		t.Fatal("different source reused browser cache")
	}
	encoded, err := json.Marshal(config)
	if err != nil || strings.Contains(string(encoded), "original") || strings.Contains(string(encoded), "RuleSource") {
		t.Fatalf("source leaked into config JSON: %s %v", encoded, err)
	}
}

func TestDirectBrowserCallerCapturesRuleOnce(t *testing.T) {
	file := filepath.Join(t.TempDir(), "rule.js")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := captureBrowserRule(define.JavaScriptConfig{File: file})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err := captureBrowserRule(config)
	if err != nil || again.RuleSource.Text() != "first" {
		t.Fatalf("snapshot reread: %#v %v", again, err)
	}
	fresh, err := captureBrowserRule(define.JavaScriptConfig{File: file})
	if err != nil || fresh.RuleSource.Text() != "second" || browserCacheKey(config, false) == browserCacheKey(fresh, false) {
		t.Fatal("fresh direct call did not capture edited rule")
	}
	if _, err := captureBrowserRule(define.JavaScriptConfig{File: file + ".missing"}); err == nil {
		t.Fatal("missing rule accepted")
	}
}
