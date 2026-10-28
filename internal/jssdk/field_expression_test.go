package jssdk_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestGoAndJavaScriptFieldExpressionClassification(t *testing.T) {
	values := []string{"固定作者", "固定分类", "Yahoo", "h2", "ARTICLE", "custom-card", "Author Name", "a[data-id]", ".entry a", "a:first-child", "css:custom-card", "css:", "literal:a", "literal:Fixed author", "literal:https://example.org", "literal:", "", "x\u00a0y"}
	input, _ := json.Marshal(values)
	output, err := jssdk.RunCode(jssdk.TPL_SSR_JS, fmt.Sprintf("JSON.stringify(%s.map(RSSCanFieldExpression))", input), "field-contract.js")
	if err != nil {
		t.Fatal(err)
	}
	var results []struct {
		Value    string `json:"value"`
		Selector bool   `json:"selector"`
		Explicit bool   `json:"explicit"`
	}
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatal(err)
	}
	for i, value := range values {
		want := define.ParseFieldExpression(value)
		got := results[i]
		if got.Value != want.Value || got.Selector != want.Selector || got.Explicit != want.Explicit {
			t.Fatalf("%q Go=%+v JS=%+v", value, want, got)
		}
	}
}
