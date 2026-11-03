package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/soulteary/RSS-Can/internal/application"
	"github.com/soulteary/RSS-Can/internal/cmd"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/version"
)

func main() {
	logger.Initialize()
	if err := run(); err != nil {
		logStartupFailure()
		os.Exit(1)
	}
}
func run() error {
	doctor := len(os.Args) > 1 && os.Args[1] == "doctor"
	if doctor {
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	cmd.ApplyFlags()
	if define.DEBUG_MODE {
		logger.SetLevel("debug")
	} else {
		logger.SetLevel(define.DEFAULT_DEBUG_LEVEL)
	}
	logger.Instance.Infof("version: %v commit: %v build: %v", version.Version, version.Commit, version.BuildDate)
	config, err := application.ConfigFromGlobals()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if doctor {
		failed := false
		for _, result := range application.Diagnose(ctx, config) {
			status := "OK"
			if !result.OK {
				status = "FAIL"
				failed = true
			}
			fmt.Fprintf(os.Stdout, "%s %s: %s\n", status, result.Name, result.Message)
		}
		if failed {
			return errors.New("startup diagnostics failed")
		}
		return nil
	}
	app, err := application.New(ctx, config)
	if err != nil {
		return err
	}
	return app.Run(ctx)
}

// Startup failures can include rule source snippets or private upstream URLs.
func logStartupFailure() {
	logger.Instance.Error("RSS CAN stopped; check configuration and dependency readiness")
}
