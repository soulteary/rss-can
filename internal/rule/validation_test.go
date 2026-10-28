package rule

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestConfigRejectsInvalidParserSettings(t *testing.T) {
	for _, tc := range []struct{ body, field string }{
		{`{"IdByRegexp":"["}`, "IdByRegexp"},
		{`{"IdByRegexp":"/post/[0-9]+"}`, "IdByRegexp"},
		{`{"PagerLimit":-1}`, "PagerLimit"},
		{`{"PagerLimit":21}`, "PagerLimit"},
	} {
		_, err := ParseConfigFromJSON(tc.body, define.RuleCache{})
		if err == nil || !strings.Contains(err.Error(), tc.field) {
			t.Fatalf("%s: %v", tc.body, err)
		}
	}
	if _, err := ParseConfigFromJSON(`{"IdByRegexp":"/post/([0-9]+)","PagerLimit":0}`, define.RuleCache{}); err != nil {
		t.Fatal(err)
	}
}
