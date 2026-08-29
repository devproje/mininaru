// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"os"
	"path/filepath"
	"strings"
)

const gitHeadPrefix = "ref: refs/heads/"
const gitShortHashLen = 7

func resolveGitFile(base, target string) string {
	if filepath.IsAbs(target) {
		return target
	}

	return filepath.Join(base, target)
}

func gitDirFrom(base string) string {
	var info os.FileInfo
	var buf []byte
	var content string

	var err error

	info, err = os.Stat(filepath.Join(base, ".git"))
	if err != nil {
		return ""
	}

	if info.IsDir() {
		return filepath.Join(base, ".git")
	}

	buf, err = os.ReadFile(filepath.Join(base, ".git"))
	if err != nil {
		return ""
	}

	content = strings.TrimSpace(string(buf))
	if !strings.HasPrefix(content, "gitdir: ") {
		return ""
	}

	return resolveGitFile(base, strings.TrimPrefix(content, "gitdir: "))
}

func gitDir(cwd string) string {
	var dir string
	var parent string
	var found string

	dir = cwd

	for {
		found = gitDirFrom(dir)
		if found != "" {
			return found
		}

		parent = filepath.Dir(dir)
		if parent == dir {
			return ""
		}

		dir = parent
	}
}

func gitBranchName(gitdir string) string {
	var buf []byte
	var content string

	var err error

	buf, err = os.ReadFile(filepath.Join(gitdir, "HEAD"))
	if err != nil {
		return ""
	}

	content = strings.TrimSpace(string(buf))

	if strings.HasPrefix(content, gitHeadPrefix) {
		return strings.TrimPrefix(content, gitHeadPrefix)
	}

	if len(content) >= gitShortHashLen {
		return content[:gitShortHashLen]
	}

	return content
}

func gitBranch(cwd string) string {
	var dir string

	dir = gitDir(cwd)
	if dir == "" {
		return ""
	}

	return gitBranchName(dir)
}
