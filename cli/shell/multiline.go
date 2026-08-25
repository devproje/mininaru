// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
)

var errContinuationCanceled error = errors.New("continuation canceled")

func trailingBackslash(line string) bool {
	var count int
	var i int

	for i = len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		count++
	}

	return count%2 == 1
}

func bashIncomplete(source string) bool {
	var cmd *exec.Cmd
	var stderr bytes.Buffer

	var err error

	cmd = exec.Command(bashPath(), "-n", "-c", source)
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err == nil {
		return false
	}

	return strings.Contains(stderr.String(), "unexpected EOF") || strings.Contains(stderr.String(), "unexpected end of file")
}

func incomplete(source string) bool {
	if trailingBackslash(source) {
		return true
	}

	return bashIncomplete(source)
}

func continueLine(sh *state, first string) (string, error) {
	var source string
	var next string

	var err error

	source = first

	for incomplete(source) {
		sh.continuation = true
		next, err = readLine(sh)
		sh.continuation = false

		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", err
			}

			return "", errContinuationCanceled
		}

		source = source + "\n" + next
	}

	return source, nil
}
