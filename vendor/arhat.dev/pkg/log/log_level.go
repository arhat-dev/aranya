/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
