// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/devproje/mininaru/util"
)

const (
	defaultHistSize     int = 500
	defaultHistFileSize int = 500
)

func envInt(name string, fallback int) int {
	var raw string
	var n int

	var err error

	raw = os.Getenv(name)
	if raw == "" {
		return fallback
	}

	n, err = strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}

	return n
}

func histFile() string {
	var custom string

	custom = os.Getenv("NARU_HISTFILE")
	if custom != "" {
		return custom
	}

	return util.Path("history")
}

func trimHistory(list []string, limit int) []string {
	if limit <= 0 || len(list) <= limit {
		return list
	}

	return list[len(list)-limit:]
}

func loadHistory() []string {
	var file *os.File
	var scanner *bufio.Scanner
	var line string
	var list []string

	var err error

	file, err = os.Open(histFile())
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner = bufio.NewScanner(file)

	for scanner.Scan() {
		line = scanner.Text()
		if line == "" {
			continue
		}

		list = append(list, line)
	}

	return trimHistory(list, envInt("HISTSIZE", defaultHistSize))
}

func saveHistory(list []string) {
	var err error

	list = trimHistory(list, envInt("HISTFILESIZE", defaultHistFileSize))

	err = util.WriteFileAtomic(histFile(), fmt.Appendf(nil, "%s\n", strings.Join(list, "\n")), 0600)
	if err != nil {
		util.Log.Debug("client history not saved", "error", err)
	}
}

func recordHistory(list []string, line string) []string {
	if line == "" {
		return list
	}

	if len(list) > 0 && list[len(list)-1] == line {
		return list
	}

	return trimHistory(append(list, line), envInt("HISTSIZE", defaultHistSize))
}
