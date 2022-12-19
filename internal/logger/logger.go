package logger

import (
	"os"

	"github.com/soulteary/RSS-Can/internal/define"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Instance *zap.SugaredLogger
var initMutex bool
var atom zap.AtomicLevel

func Initialize() {

	if initMutex {
		return
	}

	atom = zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = ""

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	))
	defer logger.Sync()

	if define.GLOBAL_DEBUG_MODE {
		atom.SetLevel(zap.DebugLevel)
	} else {
		atom.SetLevel(zap.WarnLevel)
	}

	Instance = logger.Sugar()
	initMutex = true

	logger.Debug("The log component is loaded")
}

func SetLevel(level string) {
	switch level {
	case "debug":
		atom.SetLevel(zap.DebugLevel)
		return
	case "info":
		atom.SetLevel(zap.InfoLevel)
		return
	case "warn":
		atom.SetLevel(zap.WarnLevel)
		return
	case "error":
		atom.SetLevel(zap.ErrorLevel)
		return
	}
}
