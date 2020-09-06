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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	TimeLayout = time.RFC3339Nano
)

var (
	Log        Interface
	NoOpLogger Interface
	once       = new(sync.Once)
)

func init() {
	var err error
	// initial nop logger
	NoOpLogger, err = New("", ConfigSet{})
	Log = NoOpLogger

	if err != nil {
		panic(err)
	}
}

func getLevelEnablerFunc(targetLevel zapcore.Level) zap.LevelEnablerFunc {
	return func(l zapcore.Level) bool {
		return l >= targetLevel
	}
}

func getEncoder(format string) zapcore.Encoder {
	var encoderConfig = zapcore.EncoderConfig{
		MessageKey:     "M",
		LevelKey:       "L",
		TimeKey:        "T",
		NameKey:        "N",
		CallerKey:      "C",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeName:     zapcore.FullNameEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeLevel: func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(Level(level).String())
		},
		EncodeCaller: func(ec zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			var caller string
			if !ec.Defined {
				caller = "undefined"
			} else {
				buf := bufferPool.Get()
				buf.AppendString(filepath.Base(ec.File))
				buf.AppendByte(':')
				buf.AppendInt(int64(ec.Line))
				caller = buf.String()
				buf.Free()
			}

			enc.AppendString(caller)
		},
	}

	switch format {
	case "json":
		encoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(t.UTC().Format(TimeLayout))
		}

		return zapcore.NewJSONEncoder(encoderConfig)
	case "console", "":
		fallthrough
	default:
		encoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(fmt.Sprintf("%-27s", t.UTC().Format(TimeLayout)))
		}
		return zapcore.NewConsoleEncoder(encoderConfig)
	}
}

func getWriteSyncer(file string) (zapcore.WriteSyncer, error) {
	switch file {
	case "stderr":
		return zapcore.Lock(os.Stderr), nil
	case "stdout":
		return zapcore.Lock(os.Stdout), nil
	default:
		fs, err := os.Stat(file)
		if err == nil && fs.Size() > 0 {
			// best effort
			_ = os.Rename(file, fmt.Sprintf("%s.old", file))
		}

		ws, _, err := zap.Open(file)
		if err != nil {
			return nil, err
		}
		return ws, nil
	}
}

func New(name string, config ConfigSet) (*Logger, error) {
	finalConfig := config.GetUnique()

	var subCores []zapcore.Core
	for _, c := range finalConfig {
		ws, err := getWriteSyncer(c.File)
		if err != nil {
			return nil, err
		}

		subCores = append(subCores,
			zapcore.NewCore(getEncoder(c.Format), ws, getLevelEnablerFunc(parseZapLevel(c.Level))),
		)
	}
	core := zapcore.NewTee(subCores...)
	return &Logger{name: name, core: core}, nil
}

func SetDefaultLogger(cs ConfigSet) error {
	var err error
	once.Do(func() {
		Log, err = New("", cs)
		if err != nil {
			panic(err)
		}
	})

	return err
}

type Logger struct {
	name string
	core zapcore.Core
}

func (l *Logger) Enabled(level Level) bool {
	return l.core.Enabled(zapcore.Level(level))
}

func (l *Logger) WithName(name string) Interface {
	return &Logger{
		name: name,
		core: l.core,
	}
}

func (l *Logger) WithFields(fields ...Field) Interface {
	return &Logger{
		name: l.name,
		core: l.core.With(fields),
	}
}

func (l *Logger) getEntry(level Level, msg string) zapcore.Entry {
	var stack string
	if level >= LevelError {
		stack = takeStacktrace()
	}

	return zapcore.Entry{
		Level:      zapcore.Level(level),
		Time:       time.Now(),
		LoggerName: l.name,
		Message:    msg,
		Caller:     zapcore.NewEntryCaller(runtime.Caller(2)),
		Stack:      stack,
	}
}

func (l *Logger) checkAndLog(entry zapcore.Entry, fields []Field) {
	checkedEntry := l.core.Check(entry, nil)
	if checkedEntry != nil {
		checkedEntry.Write(fields...)
	}
}

// V verbose
func (l *Logger) V(msg string, fields ...Field) {
	l.checkAndLog(l.getEntry(LevelVerbose, msg), fields)
}

// D debug
func (l *Logger) D(msg string, fields ...Field) {
	l.checkAndLog(l.getEntry(LevelDebug, msg), fields)
}

// I info
func (l *Logger) I(msg string, fields ...Field) {
	l.checkAndLog(l.getEntry(LevelInfo, msg), fields)
}

// E error
func (l *Logger) E(msg string, fields ...Field) {
	l.checkAndLog(l.getEntry(LevelError, msg), fields)
}

// Flush log write
func (l *Logger) Flush() error {
	return l.core.Sync()
}
