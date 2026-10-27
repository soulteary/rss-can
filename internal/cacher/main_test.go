package cacher_test

import (
	"os"
	"testing"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Initialize()
	if address := os.Getenv("RSS_SERVER"); address != "" {
		define.REDIS_SERVER = address
	}
	os.Exit(m.Run())
}
