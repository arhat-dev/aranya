// +build !js,!plan9,!aix

/*
Copyright 2017 The Kubernetes Authors.

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

package actionutil

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"
	"arhat.dev/pkg/log"
	"github.com/fsnotify/fsnotify"
)

// Notice that the current CRI logs implementation doesn't handle
// log rotation.
// * It will not retrieve logs in rotated log file.
// * If log rotation happens when following the log:
//   * If the rotation is using create mode, we'll still follow the old file.
//   * If the rotation is using copytruncate, we'll be reading at the original position and get nothing.

const (
	// timeFormat is the time format used in the log.
	timeFormat = time.RFC3339Nano
)

type streamType string

const (
	stderr streamType = "stderr"
	stdout streamType = "stdout"
)

var (
	// eol is the end-of-line sign in the log.
	eol = []byte{'\n'}
	// delimiter is the delimiter for timestamp and stream type in log line.
	delimiter = []byte{' '}
	// tagDelimiter is the delimiter for log tags.
	tagDelimiter = []byte{':'}
)

// logMessage is the CRI internal log type.
type logMessage struct {
	timestamp time.Time
	stream    streamType
	log       []byte
}

// reset resets the log to nil.
func (l *logMessage) reset() {
	l.timestamp = time.Time{}
	l.stream = ""
	l.log = nil
}

// LogOptions is the CRI internal type of all log options.
type LogOptions struct {
	tail      int64
	bytes     int64
	since     time.Time
	follow    bool
	timestamp bool
}

// parseFunc is a function parsing one log line to the internal log type.
// Notice that the caller must make sure logMessage is not nil.
type parseFunc func([]byte, *logMessage) error

var parseFuncs = []parseFunc{
	parseCRILog,        // CRI log format parse function
	parseDockerJSONLog, // Docker JSON log format parse function

	// custom log parser
	parseArhatConsoleLog,
	parseArhatJSONLog,
}

func parseArhatConsoleLog(line []byte, msg *logMessage) (err error) {
	i := bytes.IndexByte(line, '\t')
	if i < 0 {
		return fmt.Errorf("unexpected non tab existence")
	}
	msg.timestamp, err = time.Parse(log.TimeLayout, string(line[:i]))
	if err != nil {
		return fmt.Errorf("unexpected timestamp format %v: %v", string(line[:i]), err)
	}
	msg.log = line
	msg.stream = stderr
	return nil
}

func parseArhatJSONLog(line []byte, msg *logMessage) error {
	logEntry := new(log.Structure)

	if err := json.Unmarshal(line, logEntry); err != nil {
		return err
	}

	msg.log = line
	msg.timestamp, _ = time.Parse(log.TimeLayout, logEntry.Time)
	msg.stream = stderr
	return nil
}

// parseCRILog parses logs in CRI log format. CRI Log format example:
//   2016-10-06T00:17:09.669794202Z stdout P log content 1
//   2016-10-06T00:17:09.669794203Z stderr F log content 2
func parseCRILog(log []byte, msg *logMessage) error {
	var err error
	// Parse timestamp
	idx := bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("timestamp is not found")
	}
	msg.timestamp, err = time.Parse(timeFormat, string(log[:idx]))
	if err != nil {
		return fmt.Errorf("unexpected timestamp format %q: %v", timeFormat, err)
	}

	// Parse stream type
	log = log[idx+1:]
	idx = bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("stream type is not found")
	}
	msg.stream = streamType(log[:idx])
	if msg.stream != stdout && msg.stream != stderr {
		return fmt.Errorf("unexpected stream type %q", msg.stream)
	}

	// Parse log tag
	log = log[idx+1:]
	idx = bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("log tag is not found")
	}
	// Keep this forward compatible.
	tags := bytes.Split(log[:idx], tagDelimiter)
	partial := string(tags[0]) == "P"
	// Trim the tailing new line if this is a partial line.
	if partial && len(log) > 0 && log[len(log)-1] == '\n' {
		log = log[:len(log)-1]
	}

	// Get log content
	msg.log = log[idx+1:]

	return nil
}

// JSONLog is a log message, typically a single entry from a given log stream.
type JSONLog struct {
	// Log is the log message
	Log string `json:"log,omitempty"`
	// Stream is the log source
	Stream string `json:"stream,omitempty"`
	// Created is the created timestamp of log
	Created time.Time `json:"time"`
	// Attrs is the list of extra attributes provided by the user
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Reset all fields to their zero value.
func (jl *JSONLog) Reset() {
	jl.Log = ""
	jl.Stream = ""
	jl.Created = time.Time{}
	jl.Attrs = make(map[string]string)
}

// parseDockerJSONLog parses logs in Docker JSON log format. Docker JSON log format
// example:
//   {"log":"content 1","stream":"stdout","time":"2016-10-20T18:39:20.57606443Z"}
//   {"log":"content 2","stream":"stderr","time":"2016-10-20T18:39:20.57606444Z"}
func parseDockerJSONLog(log []byte, msg *logMessage) error {
	var l = &JSONLog{}
	l.Reset()

	// TODO: JSON decoding is fairly expensive, we should evaluate this.
	if err := json.Unmarshal(log, l); err != nil {
		return fmt.Errorf("failed with %v to unmarshal log %q", err, l)
	}

	if l.Stream == "" {
		return fmt.Errorf("stream type is not found")
	}

	msg.timestamp = l.Created
	msg.stream = streamType(l.Stream)
	msg.log = []byte(l.Log)
	return nil
}

// getParseFunc returns proper parse function based on the sample log line passed in.
func getParseFunc(log []byte) (parseFunc, error) {
	for _, p := range parseFuncs {
		if err := p(log, &logMessage{}); err == nil {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unsupported log format: %q", log)
}

// logWriter controls the writing into the stream based on the log options.
type logWriter struct {
	stdout io.Writer
	stderr io.Writer
	opts   *LogOptions
	remain int64
}

// errMaximumWrite is returned when all bytes have been written.
var errMaximumWrite = errors.New("maximum write")

// errShortWrite is returned when the message is not fully written.
var errShortWrite = errors.New("short write")

func newLogWriter(stdout io.Writer, stderr io.Writer, opts *LogOptions) *logWriter {
	w := &logWriter{
		stdout: stdout,
		stderr: stderr,
		opts:   opts,
		remain: math.MaxInt64, // initialize it as infinity
	}
	if opts.bytes >= 0 {
		w.remain = opts.bytes
	}
	return w
}

// writeLogs writes logs into stdout, stderr.
func (w *logWriter) write(msg *logMessage) error {
	if msg.timestamp.Before(w.opts.since) {
		// Skip the line because it's older than since
		return nil
	}
	line := msg.log
	if w.opts.timestamp {
		prefix := append([]byte(msg.timestamp.Format(timeFormat)), delimiter[0])
		line = append(prefix, line...)
	}
	// If the line is longer than the remaining bytes, cut it.
	if int64(len(line)) > w.remain {
		line = line[:w.remain]
	}
	// Get the proper stream to write to.
	var stream io.Writer
	switch msg.stream {
	case stdout:
		stream = w.stdout
	case stderr:
		stream = w.stderr
	default:
		return fmt.Errorf("unexpected stream type %q", msg.stream)
	}
	n, err := stream.Write(line)
	w.remain -= int64(n)
	if err != nil {
		return err
	}
	// If the line has not been fully written, return errShortWrite
	if n < len(line) {
		return errShortWrite
	}
	// If there are no more bytes left, return errMaximumWrite
	if w.remain <= 0 {
		return errMaximumWrite
	}
	return nil
}

// newLogOptions convert the v1.PodLogOptions to CRI internal LogOptions.
func newLogOptions(apiOpts *aranyagopb.LogsCmd) *LogOptions {
	opts := &LogOptions{
		tail:      apiOpts.TailLines,  // -1 by default which means read all logs.
		bytes:     apiOpts.BytesLimit, // -1 by default which means read all logs.
		follow:    apiOpts.Follow,
		timestamp: apiOpts.Timestamp,
	}
	if apiOpts.Since != "" {
		opts.since, _ = time.Parse(aranyagoconst.TimeLayout, apiOpts.Since)
	}
	return opts
}

// ReadLogs read the container log and redirect into stdout and stderr.
// Note that containerID is only needed when following the log, or else
// just pass in empty string "".
func ReadLogs(ctx context.Context, path string, options *aranyagopb.LogsCmd, stdout, stderr io.Writer) error {
	var (
		err    error
		opts   = newLogOptions(options)
		writer = newLogWriter(stdout, stderr, opts)
		logMsg = &logMessage{}
	)

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open log file %q: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	// Search start point based on tail line.
	start, err := findTailLineStartIndex(f, opts.tail)
	if err != nil {
		return fmt.Errorf("failed to tail %d lines of log file %q: %v", opts.tail, path, err)
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek %d in log file %q: %v", start, path, err)
	}

	exiting := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	// reset scanner
	var (
		stop    bool
		parse   parseFunc
		watcher *fsnotify.Watcher
		r       = bufio.NewReader(f)
		found   = true
	)
	for !exiting() {
		if stop {
			return nil
		}
		l, err := r.ReadBytes(eol[0])
		if err != nil {
			if err != io.EOF { // This is an real error
				return fmt.Errorf("failed to read log file %q: %v", path, err)
			}
			if opts.follow {
				// The container is not running, we got to the end of the log.
				if !found {
					return nil
				}
				// Reset seek so that if this is an incomplete line,
				// it will be read again.
				if _, err = f.Seek(-int64(len(l)), io.SeekCurrent); err != nil {
					return fmt.Errorf("failed to reset seek in log file %q: %v", path, err)
				}
				if watcher == nil {
					// Initialize the watcher if it has not been initialized yet.
					if watcher, err = fsnotify.NewWatcher(); err != nil {
						return fmt.Errorf("failed to create fsnotify watcher: %v", err)
					}
					defer func() { _ = watcher.Close() }()
					if err = watcher.Add(f.Name()); err != nil {
						return fmt.Errorf("failed to watch file %q: %v", f.Name(), err)
					}
					// If we just created the watcher, try again to read as we might have missed
					// the event.
					continue
				}

				// Wait until the next log change.
				found, err = waitLogs(ctx, watcher)
				if err != nil {
					return err
				}
				// If the container exited consume data until the next EOF
				continue
			}
			// Should stop after writing the remaining content.
			stop = true
			if len(l) == 0 {
				continue
			}
		}
		if parse == nil {
			// Initialize the log parsing function.
			parse, err = getParseFunc(l)
			if err != nil {
				return fmt.Errorf("failed to get parse function: %v", err)
			}
		}
		// Parse the log line.
		logMsg.reset()
		if err := parse(l, logMsg); err != nil {
			continue
		}
		// Write the log line into the stream.
		if err := writer.write(logMsg); err != nil {
			if err == errMaximumWrite {
				return nil
			}
			return err
		}
	}

	return nil
}

// waitLogs wait for the next log write. It returns a boolean and an error. The boolean
// indicates whether a new log is found; the error is error happens during waiting new logs.
func waitLogs(ctx context.Context, w *fsnotify.Watcher) (bool, error) {
	errRetry := 5
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case e := <-w.Events:
			// nolint:gocritic
			switch e.Op {
			case fsnotify.Write:
				return true, nil
			}
		case err := <-w.Errors:
			if errRetry == 0 {
				return false, err
			}
			errRetry--
		}
	}
}

const (
	// blockSize is the block size used in tail.
	blockSize = 1024
)

// findTailLineStartIndex returns the start of last nth line.
// * If n < 0, return the beginning of the file.
// * If n >= 0, return the beginning of last nth line.
// Notice that if the last line is incomplete (no end-of-line), it will not be counted
// as one line.
func findTailLineStartIndex(f io.ReadSeeker, n int64) (int64, error) {
	if n < 0 {
		return 0, nil
	}
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	var left, cnt int64
	buf := make([]byte, blockSize)
	for right := size; right > 0 && cnt <= n; right -= blockSize {
		left = right - blockSize
		if left < 0 {
			left = 0
			buf = make([]byte, right)
		}
		if _, err := f.Seek(left, io.SeekStart); err != nil {
			return 0, err
		}
		if _, err := f.Read(buf); err != nil {
			return 0, err
		}
		cnt += int64(bytes.Count(buf, eol))
	}
	for ; cnt > n; cnt-- {
		idx := bytes.Index(buf, eol) + 1
		buf = buf[idx:]
		left += int64(idx)
	}
	return left, nil
}
