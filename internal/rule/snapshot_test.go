package rule

import (
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestConfigPreservesImmutableRuleSource(t *testing.T) {
	body := []byte("original")
	config, err := ParseConfigFromJSON(`{"Mode":"csr"}`, define.RuleCache{File: "rule.js", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	body[0] = '!'
	if !config.RuleSource.Present() || config.RuleSource.Text() != "original" {
		t.Fatal("configuration did not preserve source")
	}
}

func TestPublishedRuleBodiesCannotBeMutatedByReaders(t *testing.T) {
	rulesMu.Lock()
	previous := rulesCache
	rulesCache = map[string]define.RuleCache{"example": {Body: []byte("original")}}
	rulesMu.Unlock()
	defer func() { rulesMu.Lock(); rulesCache = previous; rulesMu.Unlock() }()
	got, _ := Lookup("example")
	got.Body[0] = '!'
	snapshot := Snapshot()
	snapshot["example"].Body[0] = '?'
	again, _ := Lookup("example")
	if string(again.Body) != "original" {
		t.Fatal("reader changed published rule bytes")
	}
}
