package jssdk_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestRequestIsolation(t *testing.T) {
	if _, err := jssdk.RunCode("globalThis.leaked = 42; Object.prototype.leaked = 42;", "leaked", "first.js"); err != nil {
		t.Fatal(err)
	}
	value, err := jssdk.RunCode("", "typeof leaked + ':' + typeof ({}).leaked", "second.js")
	if err != nil || value != "undefined:undefined" {
		t.Fatalf("request leaked state: %q, %v", value, err)
	}
}

func TestRepositoryRulesCompatibility(t *testing.T) {
	goldenData, err := os.ReadFile("testdata/rule-config-sha256.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]string
	if err = json.Unmarshal(goldenData, &golden); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../../rules/*/*.js")
	if err != nil || len(files) == 0 {
		t.Fatalf("find rules: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(filepath.Dir(file))+"/"+filepath.Base(file), func(t *testing.T) {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			value, err := jssdk.RunCode(jssdk.TPL_SSR_JS, jssdk.GenerateGetConfigWithRule(body), file)
			if err != nil {
				t.Fatal(err)
			}
			var config map[string]any
			if err = json.Unmarshal([]byte(value), &config); err != nil {
				t.Fatal(err)
			}
			canonical, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(canonical))
			key := filepath.Base(filepath.Dir(file)) + "/" + filepath.Base(file)
			if digest != golden[key] {
				t.Fatalf("configuration differs from v8go baseline: got %s, want %s", digest, golden[key])
			}
			t.Logf("config SHA256: %s", digest)
		})
	}
}

func TestBaseTimeoutAndRecovery(t *testing.T) {
	if _, err := jssdk.RunCode("while(true) {}", "", "base.js"); err == nil {
		t.Fatal("expected base timeout")
	}
	value, err := jssdk.RunCode("var x=7", "x", "recovery.js")
	if err != nil || value != "7" {
		t.Fatalf("recovery: %q, %v", value, err)
	}
}
