//go:build unix

package modules

import (
	"os/exec"
	"syscall"
)

func bashIsolate(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func bashTerminate(command *exec.Cmd) error {
	var pid int

	if command.Process == nil {
		return nil
	}

	pid = command.Process.Pid

	return syscall.Kill(-pid, syscall.SIGKILL)
}
