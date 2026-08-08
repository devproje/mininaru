package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const defaultBashTimeout = 30
const maxBashTimeout = 120
const maxBashOutput = 65536

const bashWaitDelay = 2 * time.Second

func bashShell() (string, error) {
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

func BashExec(root string) Def {
	return Def{
		Name:        "bash_exec",
		Description: "Execute a Bash command in the process startup directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":         map[string]any{"type": "string"},
				"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": maxBashTimeout},
			},
			"required":             []string{"command"},
			"additionalProperties": false,
		},
		Permission: PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Command        string `json:"command"`
				TimeoutSeconds int    `json:"timeout_seconds"`
			}
			var workingDir string
			var shell string
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
				payload.TimeoutSeconds = defaultBashTimeout
			}
			if payload.TimeoutSeconds > maxBashTimeout {
				return "", fmt.Errorf("timeout_seconds cannot exceed %d", maxBashTimeout)
			}
			workingDir, err = toolRoot(root)
			if err != nil {
				return "", err
			}
			shell, err = bashShell()
			if err != nil {
				return "", err
			}

			commandCtx, cancel = context.WithTimeout(ctx, time.Duration(payload.TimeoutSeconds)*time.Second)
			defer cancel()
			command = exec.CommandContext(commandCtx, shell, "-lc", payload.Command)
			command.Dir = workingDir
			command.WaitDelay = bashWaitDelay
			command.Cancel = func() error { return bashTerminate(command) }
			bashIsolate(command)

			output, err = command.CombinedOutput()
			if len(output) > maxBashOutput {
				output = append(output[:maxBashOutput], []byte("\n[truncated]")...)
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
