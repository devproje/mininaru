// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package client

import (
	"os/exec"
)

func setGroup(cmd *exec.Cmd) {
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	cmd.Process.Kill()
}
