package main

import (
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/server"
	"github.com/soulteary/RSS-Can/internal/version"
)

func main() {
	logger.Initialize()

	if define.GLOBAL_DEBUG_MODE {
		logger.SetLevel("debug")
	} else {
		logger.SetLevel(define.GLOBAL_DEBUG_LEVEL)
	}

	logger.Instance.Infof("version: %v commit: %v build: %v", version.Version, version.Commit, version.BuildDate)

	server.StartWebServer()
}
