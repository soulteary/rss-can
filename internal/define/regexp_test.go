package define

import "testing"

func TestCompileIDRegexp(t *testing.T) {
	for _, tc := range []struct {
		pattern       string
		valid, exists bool
	}{{"", true, false}, {"   ", true, false}, {"[", false, false}, {"[0-9]+", false, false}, {"/post/([0-9]+)", true, true}} {
		re, err := (JavaScriptConfig{IdByRegexp: tc.pattern}).CompileIDRegexp()
		if (err == nil) != tc.valid || (re != nil) != tc.exists {
			t.Fatalf("%q: %v %v", tc.pattern, re, err)
		}
	}
}
