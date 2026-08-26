// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package shell

import (
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

func pollStdin(timeout time.Duration) bool {
	var event uint32

	var err error

	event, err = windows.WaitForSingleObject(windows.Handle(os.Stdin.Fd()), uint32(timeout.Milliseconds()))
	if err != nil {
		return false
	}

	return event == windows.WAIT_OBJECT_0
}

func setForeground(pgid int) {
}

func runForeground(cmd *exec.Cmd) error {
	var err error

	err = cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}

func enableAnsi() {
	var handle windows.Handle
	var mode uint32

	var err error

	handle, err = windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return
	}

	err = windows.GetConsoleMode(handle, &mode)
	if err != nil {
		return
	}

	windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
