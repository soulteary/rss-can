package jssdk_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func init() {
	logger.Initialize()
}

func TestRunCode(t *testing.T) {
	// test forever loops
	start := time.Now()
	_, err := jssdk.RunCode(`while(1){console.log(1)}`, "", "test.js")
	if err == nil {
		t.Fatalf("Programs executed without aborting timeouts")
	}
	duration := time.Since(start)
	fmt.Println(duration)
	if duration > (fn.I2T(define.JS_EXECUTE_TIMEOUT) * time.Millisecond * 100) {
		t.Fatalf("Code execution takes longer than expected")
	}

	// test normal code
	ret, err := jssdk.RunCode(`var a = 1;`, "", "test.js")
	if err != nil {
		t.Fatalf("Programs executed failed: %v", err)
	}
	if ret != "undefined" {
		t.Fatalf("Programs executed failed")
	}

	// test inject code
	ret, err = jssdk.RunCode(`var a = 1;`, "a", "test.js")
	if err != nil {
		t.Fatalf("Programs executed failed: %v", err)
	}
	if ret != "1" {
		t.Fatalf("Programs executed failed")
	}

	// test inject code with error
	ret, err = jssdk.RunCode(`var a = 1;`, "b", "test.js")
	if err == nil {
		t.Fatalf("Programs executed failed")
	}
	if strings.Contains(ret, "is not defined") {
		t.Fatalf("Programs executed failed")
	}
}

func TestGetCtxWithJS(t *testing.T) {
	_, err := jssdk.GetCtxWithJS("1", "test.js")
	if err != nil {
		t.Fatal("GetCtxWithJS failed")
	}

	_, err = jssdk.GetCtxWithJS("!not-found-command", "test.js")
	if err == nil {
		t.Fatal("GetCtxWithJS failed")
	}
}
