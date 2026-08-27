// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/term"
)

var subcommandSets = map[string][]string{
	"git": {"add", "branch", "checkout", "clone", "commit", "diff", "fetch",
		"init", "log", "merge", "pull", "push", "rebase", "remote",
		"reset", "restore", "revert", "rm", "show", "stash", "status",
		"switch", "tag", "worktree"},
	"go": {"build", "clean", "doc", "env", "fmt", "generate", "get",
		"install", "list", "mod", "run", "test", "tool", "vet", "work"},
	"npm": {"install", "ci", "run", "start", "test", "build", "publish",
		"init", "update", "uninstall", "list", "outdated", "audit"},
	"docker": {"build", "run", "ps", "images", "pull", "push", "exec", "logs",
		"stop", "start", "rm", "rmi", "compose", "network", "volume"},
	"cargo": {"build", "run", "test", "check", "clean", "doc", "new", "init",
		"add", "remove", "update", "publish"},
}

func wordStart(line string) int {
	var i int

	for i = len(line) - 1; i >= 0; i-- {
		if line[i] == ' ' || line[i] == '\t' {
			return i + 1
		}
	}

	return 0
}

func expandUser(path string) string {
	var home string

	var err error

	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err = os.UserHomeDir()
	if err != nil {
		return path
	}

	return home + strings.TrimPrefix(path, "~")
}

func fileCandidates(sh *state, word string) []string {
	var dir string
	var base string
	var target string
	var entries []os.DirEntry
	var entry os.DirEntry
	var name string
	var items []string

	var err error

	dir, base = "", word

	if strings.Contains(word, "/") {
		dir = word[:strings.LastIndex(word, "/")+1]
		base = word[strings.LastIndex(word, "/")+1:]
	}

	target = expandUser(dir)
	if target == "" {
		target = sh.cwd
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(sh.cwd, target)
	}

	entries, err = os.ReadDir(target)
	if err != nil {
		return nil
	}

	for _, entry = range entries {
		name = entry.Name()

		if !strings.HasPrefix(name, base) {
			continue
		}

		if base == "" && strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			name = name + "/"
		}

		items = append(items, dir+name)
	}

	sort.Strings(items)

	return items
}

func commandCandidates(word string) []string {
	var dirs []string
	var dir string
	var entries []os.DirEntry
	var entry os.DirEntry
	var seen map[string]bool
	var name string
	var info os.FileInfo
	var items []string

	var err error

	seen = map[string]bool{}

	for _, name = range []string{"cd", "exit", "quit", "history"} {
		if strings.HasPrefix(name, word) {
			seen[name] = true
			items = append(items, name)
		}
	}

	dirs = filepath.SplitList(os.Getenv("PATH"))

	for _, dir = range dirs {
		entries, err = os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry = range entries {
			name = entry.Name()

			if !strings.HasPrefix(name, word) || seen[name] {
				continue
			}

			info, err = entry.Info()
			if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
				continue
			}

			seen[name] = true
			items = append(items, name)
		}
	}

	sort.Strings(items)

	return items
}

func agentCommandCandidates(word string) []string {
	var name string
	var items []string

	for _, name = range commandNames() {
		if strings.HasPrefix("/"+name, word) {
			items = append(items, "/"+name)
		}
	}

	return items
}

func subcommandCandidates(command, word string) []string {
	var name string
	var items []string

	for _, name = range subcommandSets[command] {
		if strings.HasPrefix(name, word) {
			items = append(items, name)
		}
	}

	sort.Strings(items)

	return items
}

func candidates(sh *state, line string) []string {
	var word string
	var fields []string
	var items []string

	word = line[wordStart(line):]

	if sh.mode == MODE_BASH && wordStart(line) == 0 && !strings.Contains(word, "/") {
		return commandCandidates(word)
	}

	if sh.mode == MODE_AGENT && wordStart(line) == 0 && strings.HasPrefix(word, "/") {
		return agentCommandCandidates(word)
	}

	if sh.mode == MODE_BASH {
		fields = strings.Fields(line[:wordStart(line)])
		if len(fields) == 1 {
			items = subcommandCandidates(fields[0], word)
			if len(items) > 0 {
				return items
			}
		}
	}

	return fileCandidates(sh, word)
}

func commonPrefix(items []string) string {
	var prefix string
	var item string
	var i int

	prefix = items[0]

	for _, item = range items[1:] {
		for i = 0; i < len(prefix) && i < len(item); i++ {
			if prefix[i] != item[i] {
				break
			}
		}

		prefix = prefix[:i]
	}

	return prefix
}

func candidateName(item string) string {
	if strings.HasSuffix(item, "/") {
		return filepath.Base(item) + "/"
	}

	return filepath.Base(item)
}

func showCandidates(items []string) {
	var item string
	var name string
	var width int
	var column int
	var columns int
	var i int

	var err error

	for _, item = range items {
		if displayWidth(candidateName(item)) > width {
			width = displayWidth(candidateName(item))
		}
	}

	width = width + 2
	column, _, err = term.GetSize(int(os.Stdout.Fd()))
	if err != nil || column < width {
		column = width
	}

	columns = column / width

	write("\n")

	for i, item = range items {
		name = candidateName(item)

		if strings.HasSuffix(item, "/") {
			write("%s%s%s%s", BLUE, name, RESET, strings.Repeat(" ", width-displayWidth(name)))
		} else {
			write("%s%s", name, strings.Repeat(" ", width-displayWidth(name)))
		}

		if (i+1)%columns == 0 {
			write("\n")
		}
	}

	if len(items)%columns != 0 {
		write("\n")
	}
}

func complete(sh *state, line string, repeated bool) (string, bool) {
	var items []string
	var start int
	var word string
	var prefix string

	items = candidates(sh, line)
	if len(items) == 0 {
		return line, false
	}

	start = wordStart(line)
	word = line[start:]
	prefix = commonPrefix(items)

	if len(items) == 1 {
		if strings.HasSuffix(prefix, "/") {
			return line[:start] + prefix, false
		}

		return line[:start] + prefix + " ", false
	}

	if prefix != word {
		return line[:start] + prefix, false
	}

	if repeated {
		showCandidates(items)
		return line, true
	}

	return line, true
}
