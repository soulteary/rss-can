package javascript

import (
	"fmt"
	"os"
	"time"

	v8 "rogchap.com/v8go"
)

const JS_EXECUTE_TIMEOUT = 200 * time.Millisecond
const JS_EXECUTE_THORTTLING = 2 * time.Second

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
	timeout := time.NewTimer(JS_EXECUTE_TIMEOUT)

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
		time.Sleep(JS_EXECUTE_THORTTLING)
		return nil, err
	}
}

func RunCode(base string, export string) (string, error) {
	ctx := v8.NewContext()
	// test sandbox with this
	// safeJsSandbox(ctx, `while(1){console.log(1)}`, "main.js")
	RunCodeInSandbox(ctx, base, "base.js")
	result, err := ctx.RunScript(export, "export.js")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", result), err
}
