package main

import (
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/cmd"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/server"
	"github.com/soulteary/RSS-Can/internal/version"
)

func main() {
	logger.Initialize()
	defer jssdk.CloseRuntime()

	cmd.ApplyFlags()
	if err := jssdk.InitializeRuntime(); err != nil {
		logger.Instance.Fatalf("Unable to initialize JavaScript runtime: %v", err)
	}
	cacher.Initialize()
	defer func() {
		if err := cacher.Close(); err != nil {
			logger.Instance.Warnf("Unable to close cache: %v", err)
		}
	}()

	if define.DEBUG_MODE {
		logger.SetLevel("debug")
	} else {
		logger.SetLevel(define.DEFAULT_DEBUG_LEVEL)
	}

	logger.Instance.Infof("version: %v commit: %v build: %v", version.Version, version.Commit, version.BuildDate)

	server.StartWebServer()
}
