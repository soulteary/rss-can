package jssdk_test

import (
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestRuleExportTimeoutAndRecovery(t *testing.T) {
	previous := define.JS_EXECUTE_TIMEOUT
	define.JS_EXECUTE_TIMEOUT = 30
	defer func() { define.JS_EXECUTE_TIMEOUT = previous }()
	start := time.Now()
	_, err := jssdk.RunCode("var config = {};", "while (true) {}", "rule.js")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected rule timeout, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("rule timeout did not return promptly")
	}
	value, err := jssdk.RunCode("var value = 42;", "value", "next.js")
	if err != nil || value != "42" {
		t.Fatalf("execution after timeout: %q, %v", value, err)
	}
}

func TestDateInputCannotExecuteJavaScript(t *testing.T) {
	// Previously interpolated directly into executable source.
	_, err := jssdk.ConvertStrToUnix(`"); while (true) {}; //`)
	if err != nil {
		t.Fatal("date input was not treated as string data:", err)
	}
}
