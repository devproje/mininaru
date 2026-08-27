// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"bufio"
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

func histSize() int {
	return envInt("HISTSIZE", defaultHistSize)
}

func histFileSize() int {
	return envInt("HISTFILESIZE", defaultHistFileSize)
}

func histFile() string {
	var custom string

	custom = os.Getenv("HISTFILE")
	if custom != "" {
		return custom
	}

	return util.Path("shell_history")
}

func trimHistory(list []string, limit int) []string {
	if limit <= 0 || len(list) <= limit {
		return list
	}

	return list[len(list)-limit:]
}

func readHistoryFile(path string) []string {
	var file *os.File
	var scanner *bufio.Scanner
	var line string
	var list []string

	var err error

	file, err = os.Open(path)
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

	return list
}

func writeHistoryFile(path string, list []string) error {
	return util.WriteFileAtomic(path, []byte(strings.Join(list, "\n")+"\n"), 0600)
}

func loadHistory(sh *state) {
	sh.history = trimHistory(readHistoryFile(histFile()), histSize())
}

func saveHistory(sh *state) {
	var err error

	err = writeHistoryFile(histFile(), trimHistory(sh.history, histFileSize()))
	if err != nil {
		util.Log.Debug("shell history not saved", "error", err)
	}
}

func historyFor(sh *state) []string {
	if sh.mode == MODE_AGENT {
		return sh.agentHistory
	}

	return sh.history
}

func recordHistory(sh *state, line string) {
	if sh.mode == MODE_AGENT {
		sh.agentHistory = trimHistory(append(sh.agentHistory, line), histSize())
		return
	}

	sh.history = trimHistory(append(sh.history, line), histSize())
}

func deleteHistoryEntry(sh *state, offset int) bool {
	var index int

	index = offset - 1
	if offset < 0 {
		index = len(sh.history) + offset
	}

	if index < 0 || index >= len(sh.history) {
		return false
	}

	sh.history = append(sh.history[:index], sh.history[index+1:]...)

	return true
}

func listHistory(sh *state, count int) {
	var start int
	var i int
	var line string

	start = 0
	if count > 0 && count < len(sh.history) {
		start = len(sh.history) - count
	}

	for i, line = range sh.history[start:] {
		write("%s%5d%s  %s\n", DIM, start+i+1, RESET, line)
	}
}

func historyFileArg(args []string) string {
	if len(args) > 2 {
		return args[2]
	}

	return histFile()
}

func printHistory(sh *state, args []string) {
	var count int
	var path string

	var err error

	if len(args) == 1 {
		listHistory(sh, 0)
		return
	}

	switch args[1] {
	case "-c":
		sh.history = nil
		return
	case "-w":
		err = writeHistoryFile(historyFileArg(args), sh.history)
		if err != nil {
			notice(RED, "✖", "history: %v", err)
		}

		return
	case "-r":
		path = historyFileArg(args)
		sh.history = trimHistory(append(sh.history, readHistoryFile(path)...), histSize())
		return
	case "-d":
		if len(args) < 3 {
			notice(RED, "✖", "history: -d: option requires an argument")
			return
		}

		count, err = strconv.Atoi(args[2])
		if err != nil {
			notice(RED, "✖", "history: %s: numeric argument required", args[2])
			return
		}

		if !deleteHistoryEntry(sh, count) {
			notice(RED, "✖", "history: %s: history position out of range", args[2])
		}

		return
	}

	count, err = strconv.Atoi(args[1])
	if err != nil {
		notice(RED, "✖", "history: %s: numeric argument required", args[1])
		return
	}

	listHistory(sh, count)
}
