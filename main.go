package main

import (
	"github.com/soulteary/RSS-Can/internal/server"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func main() {
	logger.Initialize()
	server.ServAPI()
}
