package javascript

import (
	"fmt"
	"os"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"

	v8 "rogchap.com/v8go"
)

func RunCodeInSandbox(ctx *v8.Context, unsafe string, fileName string) (*v8.Value, error) {
	vals := make(chan *v8.Value, 1)
	errs := make(chan error, 1)

	start := time.Now()
	go func() {
		val, err := ctx.RunScript(unsafe, fileName)
		if err != nil {
			errs <- err
			return
		}
		vals <- val
	}()

	duration := time.Since(start)
	timeout := time.NewTimer(fn.I2T(define.DEFAULT_JS_EXECUTE_TIMEOUT) * time.Millisecond)

	select {
	case val := <-vals:
		if !timeout.Stop() {
			<-timeout.C
		}
		fmt.Fprintf(os.Stderr, "cost time: %v\n", duration)
		return val, nil
	case err := <-errs:
		return nil, err
	case <-timeout.C:
		timeout.Stop()
		vm := ctx.Isolate()
		vm.TerminateExecution()
		err := <-errs
		fmt.Fprintf(os.Stderr, "execution timeout: %v\n", duration)
		time.Sleep(fn.I2T(define.JS_EXECUTE_THORTTLING) * time.Second)
		return nil, err
	}
}

func RunCode(base string, export string) (string, error) {
	ctx := v8.NewContext()
	_, err := RunCodeInSandbox(ctx, base, "base.js")
	if err != nil {
		return "", err
	}
	result, err := ctx.RunScript(export, "export.js")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", result), err
}
