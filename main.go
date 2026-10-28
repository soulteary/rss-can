package main

import (
	"context"
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
		logger.Instance.Errorf("RSS CAN stopped: %v", err)
		os.Exit(1)
	}
}
func run() error {
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
	app, err := application.New(ctx, config)
	if err != nil {
		return err
	}
	return app.Run(ctx)
}
