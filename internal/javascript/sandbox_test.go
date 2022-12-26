package javascript_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/javascript"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func init() {
	logger.Initialize()
}

func TestRunCode(t *testing.T) {
	// test forever loops
	start := time.Now()
	_, err := javascript.RunCode(`while(1){console.log(1)}`, "")
	if err == nil {
		t.Fatalf("Programs executed without aborting timeouts")
	}
	duration := time.Since(start)
	fmt.Println(duration)
	if duration > (fn.I2T(define.JS_EXECUTE_TIMEOUT) * time.Millisecond * 100) {
		t.Fatalf("Code execution takes longer than expected")
	}

	// test normal code
	ret, err := javascript.RunCode(`var a = 1;`, "")
	if err != nil {
		t.Fatalf("Programs executed failed: %v", err)
	}
	if ret != "undefined" {
		t.Fatalf("Programs executed failed")
	}

	// test inject code
	ret, err = javascript.RunCode(`var a = 1;`, "a")
	if err != nil {
		t.Fatalf("Programs executed failed: %v", err)
	}
	if ret != "1" {
		t.Fatalf("Programs executed failed")
	}

	// test inject code with error
	ret, err = javascript.RunCode(`var a = 1;`, "b")
	if err == nil {
		t.Fatalf("Programs executed failed")
	}
	if strings.Contains(ret, "is not defined") {
		t.Fatalf("Programs executed failed")
	}
}
