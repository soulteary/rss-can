package javascript

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestRunCode(t *testing.T) {
	// test forever loops
	start := time.Now()
	_, err := RunCode(`while(1){console.log(1)}`, "")
	if err == nil {
		t.Fatalf("Programs executed without aborting timeouts")
	}
	duration := time.Since(start)
	if duration > (fn.I2T(define.DEFAULT_JS_EXECUTE_TIMEOUT) * time.Millisecond * 100) {
		t.Fatalf("Code execution takes longer than expected")
	}
}
