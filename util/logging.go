// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type LogOptions struct {
	Level  string
	Format string
	Output io.Writer
}

type logWriter struct {
	mu      sync.Mutex
	target  io.Writer
	held    *bytes.Buffer
	dropped int
}

const (
	LogLevelEnv  = "MININARU_LOG_LEVEL"
	LogFormatEnv = "MININARU_LOG_FORMAT"
)

const (
	LogFormatAuto = "auto"
	LogFormatText = "text"
	LogFormatJSON = "json"
)

const maxHeldLogBytes = 1 << 20

var Log *slog.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

var logSink *logWriter

func LogLevels() []string {
	return []string{"debug", "info", "warn", "error"}
}

func LogFormats() []string {
	return []string{LogFormatAuto, LogFormatText, LogFormatJSON}
}

func (w *logWriter) Write(buf []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.held == nil {
		return w.target.Write(buf)
	}

	if w.held.Len()+len(buf) > maxHeldLogBytes {
		w.dropped++
		return len(buf), nil
	}

	return w.held.Write(buf)
}

func (w *logWriter) suspend() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.held != nil {
		return
	}

	w.held = &bytes.Buffer{}
	w.dropped = 0
}

func (w *logWriter) resume() {
	var held *bytes.Buffer
	var dropped int

	w.mu.Lock()
	held = w.held
	dropped = w.dropped
	w.held = nil
	w.dropped = 0
	w.mu.Unlock()

	if held == nil {
		return
	}

	if held.Len() > 0 {
		w.mu.Lock()
		w.target.Write(held.Bytes())
		w.mu.Unlock()
	}

	if dropped > 0 {
		Log.Warn("dropped log records while the interactive client held the terminal", "records", dropped)
	}
}

func logLevel(name string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}

	return slog.LevelInfo, fmt.Errorf("unknown log level %q, expected one of %s", name, strings.Join(LogLevels(), ", "))
}

func logTerminal(out io.Writer) bool {
	var file *os.File
	var ok bool
	var info os.FileInfo

	var err error

	file, ok = out.(*os.File)
	if !ok {
		return false
	}

	info, err = file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func logFormat(name string, out io.Writer) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case LogFormatText:
		return LogFormatText, nil
	case LogFormatJSON:
		return LogFormatJSON, nil
	case "", LogFormatAuto:
	default:
		return "", fmt.Errorf("unknown log format %q, expected one of %s", name, strings.Join(LogFormats(), ", "))
	}

	if logTerminal(out) {
		return LogFormatText, nil
	}

	return LogFormatJSON, nil
}

func unreserve(groups []string, attr slog.Attr) slog.Attr {
	var ok bool

	if len(groups) > 0 {
		return attr
	}

	switch attr.Key {
	case slog.TimeKey:
		if attr.Value.Kind() == slog.KindTime {
			return attr
		}
	case slog.LevelKey:
		_, ok = attr.Value.Any().(slog.Level)
		if ok {
			return attr
		}
	case slog.SourceKey:
		_, ok = attr.Value.Any().(*slog.Source)
		if ok {
			return attr
		}
	default:
		return attr
	}

	attr.Key = "attr_" + attr.Key

	return attr
}

func NewLog(opts LogOptions) error {
	var level slog.Level
	var format string
	var handlerOpts slog.HandlerOptions
	var handler slog.Handler

	var err error

	if opts.Output == nil {
		opts.Output = os.Stderr
	}

	if opts.Level == "" {
		opts.Level = os.Getenv(LogLevelEnv)
	}

	if opts.Format == "" {
		opts.Format = os.Getenv(LogFormatEnv)
	}

	level, err = logLevel(opts.Level)
	if err != nil {
		return err
	}

	format, err = logFormat(opts.Format, opts.Output)
	if err != nil {
		return err
	}

	logSink = &logWriter{target: opts.Output}
	handlerOpts = slog.HandlerOptions{Level: level, ReplaceAttr: unreserve}

	if format == LogFormatJSON {
		handler = slog.NewJSONHandler(logSink, &handlerOpts)
	} else {
		handler = slog.NewTextHandler(logSink, &handlerOpts)
	}

	Log = slog.New(handler)
	slog.SetDefault(Log)

	return nil
}

func LogHold() func() {
	if logSink == nil {
		return func() {}
	}

	logSink.suspend()

	return logSink.resume
}
