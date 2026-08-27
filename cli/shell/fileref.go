// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxFileRefBytes = 65536

var fileRefPattern = regexp.MustCompile(`(^|\s)@(\S+)`)

func fileReferencePaths(line string) []string {
	var matches [][]string
	var match []string
	var seen map[string]bool
	var paths []string

	matches = fileRefPattern.FindAllStringSubmatch(line, -1)
	seen = make(map[string]bool)

	for _, match = range matches {
		if seen[match[2]] {
			continue
		}

		seen[match[2]] = true
		paths = append(paths, match[2])
	}

	return paths
}

func readFileReference(sh *state, rel string) (string, error) {
	var target string
	var buf []byte

	var err error

	target = expandUser(rel)
	if !filepath.IsAbs(target) {
		target = filepath.Join(sh.cwd, target)
	}

	buf, err = os.ReadFile(target)
	if err != nil {
		return "", err
	}

	if len(buf) > maxFileRefBytes {
		buf = append(buf[:maxFileRefBytes:maxFileRefBytes], []byte("\n[truncated]")...)
	}

	return string(buf), nil
}

func expandFileReferences(sh *state, line string) string {
	var paths []string
	var path string
	var content string
	var attachments []string
	var result string

	var err error

	paths = fileReferencePaths(line)
	if len(paths) == 0 {
		return line
	}

	result = fileRefPattern.ReplaceAllString(line, "${1}${2}")

	for _, path = range paths {
		content, err = readFileReference(sh, path)
		if err != nil {
			continue
		}

		attachments = append(attachments, fmt.Sprintf("<file path=%q>\n%s\n</file>", path, content))
	}

	if len(attachments) == 0 {
		return result
	}

	return result + "\n\n" + strings.Join(attachments, "\n\n")
}
