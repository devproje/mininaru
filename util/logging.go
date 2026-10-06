// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type LogOptions struct {
	Level  string
	Format string
	Output io.Writer
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

var Log *slog.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

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

	return slog.LevelInfo, fmt.Errorf("unknown log level %q, expected one of %s", name, "debug, info, warn, error")
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
		return "", fmt.Errorf("unknown log format %q, expected one of %s", name, "auto, text, json")
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

	handlerOpts = slog.HandlerOptions{Level: level, ReplaceAttr: unreserve}

	if format == LogFormatJSON {
		handler = slog.NewJSONHandler(opts.Output, &handlerOpts)
	} else {
		handler = slog.NewTextHandler(opts.Output, &handlerOpts)
	}

	Log = slog.New(handler)
	slog.SetDefault(Log)

	return nil
}
