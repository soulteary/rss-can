package javascript

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
)

func TestRunCode(t *testing.T) {
	// test forever loops
	start := time.Now()
	_, err := RunCode(`while(1){console.log(1)}`, "")
	if err != nil {
		t.Fatalf("Error parsing and executing code")
	}
	duration := time.Since(start)
	if duration > (define.JS_EXECUTE_TIMEOUT * 100) {
		t.Fatalf("Code execution takes longer than expected")
	}
}
