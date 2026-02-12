// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * MiniNaru
 * Copyright (C) 2022-2026 Project_IO
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package log

import (
	"fmt"
	"io"
	"os"
	"time"

	"git.wh64.net/naru-studio/mininaru/config"
)

type LoggingLevel uint

type LoggerSubsystem struct {
	writer io.Writer
}

var (
	LOG_EMERG   LoggingLevel = 0
	LOG_ALERT   LoggingLevel = 1
	LOG_CRIT    LoggingLevel = 2
	LOG_ERR     LoggingLevel = 3
	LOG_WARNING LoggingLevel = 4
	LOG_NOTICE  LoggingLevel = 5
	LOG_INFO    LoggingLevel = 6
	LOG_DEBUG   LoggingLevel = 7
)

var module *LoggerSubsystem = nil

func Init() error {
	var err error
	var file io.Writer
	var cnf = config.Get
	var path = fmt.Sprintf("%s/latest.log", cnf.DataDir)

	module = &LoggerSubsystem{}

	if _, err = os.Stat(path); err != nil {
		err = os.WriteFile(path, nil, 0644)
		if err != nil {
			goto err_log_handle
		}

		err = nil
	}

	file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		goto err_log_handle
	}

	module.writer = io.MultiWriter(os.Stdout, file)

	return nil

err_log_handle:
	return err
}

func Destroy() error {
	if module.writer != nil {
		module.writer = nil
	}

	return nil
}

func Logf(level LoggingLevel, format string, args ...any) (int, error) {
	var n int
	var err error
	var prefix string = ""
	var date = time.Now()

	switch level {
	case LOG_EMERG:
		prefix = "EMERG"
	case LOG_ALERT:
		prefix = "ALERT"
	case LOG_CRIT:
		prefix = "CRIT"
	case LOG_ERR:
		prefix = "ERROR"
	case LOG_WARNING:
		prefix = "WRAN"
	case LOG_NOTICE:
		prefix = "NOTICE"
	case LOG_INFO:
		prefix = "INFO"
	case LOG_DEBUG:
		prefix = "DEBUG"
	}

	n, err = fmt.Fprintf(module.writer, "%s %-10s %s",
		date.Format(time.RFC3339),
		prefix,
		fmt.Sprintf(format, args...))
	if err != nil {
		return n, err
	}

	return n, nil
}

func Emergf(format string, args ...any) (int, error) {
	return Logf(LOG_EMERG, format, args...)
}

func Alertf(format string, args ...any) (int, error) {
	return Logf(LOG_ALERT, format, args...)
}

func Critf(format string, args ...any) (int, error) {
	return Logf(LOG_CRIT, format, args...)
}

func Errorf(format string, args ...any) (int, error) {
	return Logf(LOG_ERR, format, args...)
}

func Warnf(format string, args ...any) (int, error) {
	return Logf(LOG_WARNING, format, args...)
}

func Noticef(format string, args ...any) (int, error) {
	return Logf(LOG_NOTICE, format, args...)
}

func Infof(format string, args ...any) (int, error) {
	return Logf(LOG_INFO, format, args...)
}

func Debugf(format string, args ...any) (int, error) {
	return Logf(LOG_DEBUG, format, args...)
}

func Printf(format string, args ...any) (int, error) {
	return Infof(format, args...)
}
