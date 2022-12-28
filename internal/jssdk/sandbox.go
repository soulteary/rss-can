package jssdk

import (
	"fmt"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
	v8 "rogchap.com/v8go"
)

func RunCodeInSandbox(ctx *v8.Context, unsafe string, fileName string) (*v8.Value, time.Duration, error) {
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
	timeout := time.NewTimer(fn.I2T(define.JS_EXECUTE_TIMEOUT) * time.Millisecond)

	select {
	case val := <-vals:
		if !timeout.Stop() {
			<-timeout.C
		}
		logger.Instance.Infof("Parsing config successed, cost time: %v", duration)
		return val, duration, nil
	case err := <-errs:
		return nil, duration, err
	case <-timeout.C:
		timeout.Stop()
		vm := ctx.Isolate()
		vm.TerminateExecution()
		err := <-errs
		logger.Instance.Infof("execution timeout: %v", duration)
		time.Sleep(fn.ExpireBySecond(define.DEFAULT_JS_EXECUTE_THORTTLING))
		return nil, duration, err
	}
}

func GetCtxWithJS(basejs string) (*v8.Context, error) {
	ctx := v8.NewContext()
	_, _, err := RunCodeInSandbox(ctx, basejs, "base.js")
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func RunCode(base string, export string) (string, error) {
	ctx, err := GetCtxWithJS(base)
	if err != nil {
		return "", err
	}
	result, err := ctx.RunScript(export, "export.js")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", result), err
}
