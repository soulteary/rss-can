package jssdk

import (
	"fmt"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	v8 "rogchap.com/v8go"
)

func RunCodeInSandbox(ctx *v8.Context, source string, fileName string) (*v8.Value, time.Duration, error) {
	type execution struct {
		value *v8.Value
		err   error
	}
	done := make(chan execution, 1)
	start := time.Now()
	go func() {
		value, err := ctx.RunScript(source, fileName)
		done <- execution{value, err}
	}()
	timeout := time.NewTimer(time.Duration(define.JS_EXECUTE_TIMEOUT) * time.Millisecond)
	defer timeout.Stop()
	select {
	case result := <-done:
		return result.value, time.Since(start), result.err
	case <-timeout.C:
		ctx.Isolate().TerminateExecution()
		// Join the worker before its context/isolate can be disposed. A single result
		// channel also handles a script finishing at the same instant as the timer.
		<-done
		return nil, time.Since(start), fmt.Errorf("JavaScript execution timed out: %s", fileName)
	}
}

// CloseContext releases both the context and the isolate created by GetCtxWithJS.
func CloseContext(ctx *v8.Context) {
	isolate := ctx.Isolate()
	ctx.Close()
	isolate.Dispose()
}

func GetCtxWithJS(basejs string, originFile string) (*v8.Context, error) {
	ctx := v8.NewContext()
	_, _, err := RunCodeInSandbox(ctx, basejs, originFile)
	if err != nil {
		CloseContext(ctx)
		return nil, err
	}
	return ctx, nil
}

func RunCode(base string, export string, originFile string) (string, error) {
	ctx, err := GetCtxWithJS(base, originFile)
	if err != nil {
		return "", err
	}
	defer CloseContext(ctx)
	result, _, err := RunCodeInSandbox(ctx, export, originFile)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(result), nil
}
