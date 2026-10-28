package define

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
)

func TestRuleSourceOwnsBytesAndKeepsIdentityPrivate(t *testing.T) {
	var absent RuleSource
	if absent.Present() {
		t.Fatal("zero source must remain distinguishable from an empty rule")
	}
	body := []byte("source")
	source := NewRuleSource(body)
	body[0] = '!'
	if !source.Present() || source.Text() != "source" || source.Hash() != sha256.Sum256([]byte("source")) {
		t.Fatal("rule identity changed with caller bytes")
	}
	if empty := NewRuleSource(nil); !empty.Present() || empty.Text() != "" || empty.Hash() != sha256.Sum256(nil) {
		t.Fatal("empty rule lost identity")
	}
	encoded, err := json.Marshal(JavaScriptConfig{RuleSource: source})
	if err != nil || strings.Contains(string(encoded), "source") || strings.Contains(string(encoded), "RuleSource") {
		t.Fatalf("private source leaked: %s %v", encoded, err)
	}
}
