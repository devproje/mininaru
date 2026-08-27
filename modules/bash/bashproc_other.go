// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package bash

import (
	"os/exec"
)

func isolate(command *exec.Cmd) {
}

func terminate(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}

	return command.Process.Kill()
}
