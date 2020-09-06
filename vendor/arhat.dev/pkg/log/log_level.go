package log

import (
	"strings"

	"go.uber.org/zap/zapcore"
)

type Level zapcore.Level

const (
	LevelVerbose = Level(zapcore.DebugLevel)
	LevelDebug   = Level(zapcore.InfoLevel)
	LevelInfo    = Level(zapcore.WarnLevel)
	LevelError   = Level(zapcore.ErrorLevel)
	LevelSilent  = Level(zapcore.FatalLevel + 1)
)

var levelNameMapping = map[Level]string{
	LevelVerbose: "V",
	LevelDebug:   "D",
	LevelInfo:    "I",
	LevelError:   "E",
	LevelSilent:  "S",
}

func (l Level) String() string {
	return levelNameMapping[l]
}

var strLevelToLevel = map[string]Level{
	"verbose": LevelVerbose,
	"debug":   LevelDebug,
	"info":    LevelInfo,
	"error":   LevelError,
	"silent":  LevelSilent,
}

func parseZapLevel(levelStr string) zapcore.Level {
	return zapcore.Level(strLevelToLevel[strings.ToLower(strings.TrimSpace(levelStr))])
}
