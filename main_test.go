package main

import (
	"github.com/soulteary/RSS-Can/internal/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"strings"
	"testing"
)

func TestStartupFailureMessageIsSafe(t *testing.T) {
	previous := logger.Instance
	defer func() { logger.Instance = previous }()
	core, logs := observer.New(zap.InfoLevel)
	logger.Instance = zap.New(core).Sugar()
	logStartupFailure()
	if logs.Len() != 1 || strings.Contains(logs.All()[0].Message, "%v") {
		t.Fatal("unsafe startup failure")
	}
}
