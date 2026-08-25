// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package bash

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/devproje/mininaru/modules"
)

const defaultTimeout = 30
const maxTimeout = 120
const maxOutput = 65536

const waitDelay = 2 * time.Second

func shell() (string, error) {
	var candidate string
	var candidates []string
	var resolved string

	var err error

	candidate = os.Getenv("MININARU_SHELL")
	if candidate != "" {
		return exec.LookPath(candidate)
	}

	candidates = []string{"bash", "sh"}

	for _, candidate = range candidates {
		resolved, err = exec.LookPath(candidate)
		if err != nil {
			continue
		}

		return resolved, nil
	}

	return "", fmt.Errorf("no usable shell found, set MININARU_SHELL to one")
}

func Exec(root string) modules.Tool {
	return modules.Tool{
		Name:        "bash_exec",
		Description: "Execute a Bash command in the working directory. Use file_read, file_edit, and file_write for file contents.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":         map[string]any{"type": "string"},
				"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": maxTimeout},
			},
			"required":             []string{"command"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Command        string `json:"command"`
				TimeoutSeconds int    `json:"timeout_seconds"`
			}
			var binary string
			var commandCtx context.Context
			var cancel context.CancelFunc
			var command *exec.Cmd
			var output []byte

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Command == "" {
				return "", fmt.Errorf("command is required")
			}
			if payload.TimeoutSeconds <= 0 {
				payload.TimeoutSeconds = defaultTimeout
			}
			if payload.TimeoutSeconds > maxTimeout {
				return "", fmt.Errorf("timeout_seconds cannot exceed %d", maxTimeout)
			}
			binary, err = shell()
			if err != nil {
				return "", err
			}

			commandCtx, cancel = context.WithTimeout(ctx, time.Duration(payload.TimeoutSeconds)*time.Second)
			defer cancel()
			command = exec.CommandContext(commandCtx, binary, "-c", payload.Command)
			command.Dir = root
			command.WaitDelay = waitDelay
			command.Cancel = func() error { return terminate(command) }
			isolate(command)

			output, err = command.CombinedOutput()
			if len(output) > maxOutput {
				output = append(output[:maxOutput], []byte("\n[truncated]")...)
			}
			if commandCtx.Err() == context.DeadlineExceeded {
				return string(output), fmt.Errorf("command timed out after %d seconds", payload.TimeoutSeconds)
			}
			if err != nil {
				return string(output), fmt.Errorf("command failed: %w", err)
			}

			return string(output), nil
		},
	}
}
