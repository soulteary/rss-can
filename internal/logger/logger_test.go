package logger_test

import (
	"testing"

	"github.com/soulteary/RSS-Can/internal/logger"
)

func TestInitialize(t *testing.T) {
	logger.Initialize()
	if logger.Instance == nil {
		t.Fatal("Logger instance should not be nil after initialization")
	}
}

func TestSetLevel(t *testing.T) {
	logger.Initialize()

	testCases := []struct {
		level         string
		expectedLevel string
	}{
		{"DEBUG", "DEBUG"},
		{"INFO", "INFO"},
		{"WARN", "WARN"},
		{"ERROR", "ERROR"},
		{"INVAILD", "ERROR"},
	}

	for _, tc := range testCases {
		logger.SetLevel(tc.level)
		if logger.GetLevel() != tc.expectedLevel {
			t.Fatal("Logger level should be set correctly")
		}
	}
}
