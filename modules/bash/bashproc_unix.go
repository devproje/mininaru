// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package bash

import (
	"os/exec"
	"syscall"
)

func isolate(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminate(command *exec.Cmd) error {
	var pid int

	if command.Process == nil {
		return nil
	}

	pid = command.Process.Pid

	return syscall.Kill(-pid, syscall.SIGKILL)
}
