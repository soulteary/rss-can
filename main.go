package main

import (
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/server"
)

func main() {
	logger.Initialize()

	if define.GLOBAL_DEBUG_MODE {
		logger.SetLevel("debug")
	} else {
		logger.SetLevel(define.GLOBAL_DEBUG_LEVEL)
	}

	server.ServAPI()
}
