// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func pollStdin(timeout time.Duration) bool {
	var fds []unix.PollFd
	var n int

	var err error

	fds = []unix.PollFd{{Fd: int32(os.Stdin.Fd()), Events: unix.POLLIN}}

	n, err = unix.Poll(fds, int(timeout.Milliseconds()))
	if err != nil || n <= 0 {
		return false
	}

	return fds[0].Revents&unix.POLLIN != 0
}

func setForeground(pgid int) {
	unix.IoctlSetPointerInt(int(os.Stdin.Fd()), unix.TIOCSPGRP, pgid)
}

func runForeground(cmd *exec.Cmd) error {
	var own int

	var err error

	own = unix.Getpgrp()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN)
	defer signal.Reset(syscall.SIGTTOU, syscall.SIGTTIN)

	err = cmd.Start()
	if err != nil {
		return err
	}

	setForeground(cmd.Process.Pid)

	err = cmd.Wait()

	setForeground(own)

	return err
}
