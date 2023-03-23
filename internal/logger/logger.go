package logger

import (
	"os"
	"strings"

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

	atom.SetLevel(zap.ErrorLevel)

	Instance = logger.Sugar()
	initMutex = true

	logger.Debug("The log component is loaded")
}

func SetLevel(level string) {
	newLevel := strings.ToUpper(level)
	if !((newLevel == "DEBUG") || (newLevel == "INFO") || (newLevel == "WARN") || (newLevel == "ERROR")) {
		return
	}

	switch newLevel {
	case "DEBUG":
		atom.SetLevel(zap.DebugLevel)
		return
	case "INFO":
		atom.SetLevel(zap.InfoLevel)
		return
	case "WARN":
		atom.SetLevel(zap.WarnLevel)
		return
	case "ERROR":
		atom.SetLevel(zap.ErrorLevel)
		return
	}
}

func GetLevel() string {
	return atom.Level().CapitalString()
}
