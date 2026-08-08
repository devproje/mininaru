//go:build !unix

package modules

import (
	"os/exec"
)

func bashIsolate(command *exec.Cmd) {
}

func bashTerminate(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}

	return command.Process.Kill()
}
