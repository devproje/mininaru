// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package bash

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecUsesRoot(t *testing.T) {
	var root string
	var result string

	var err error

	root = t.TempDir()
	result, err = Exec(root).Execute(context.Background(), `{"command":"pwd"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != filepath.Clean(root) {
		t.Fatalf("pwd = %q, want %q", result, root)
	}
}

func TestExecKillsBackgroundedChildOnTimeout(t *testing.T) {
	var started time.Time
	var elapsed time.Duration
	var result string

	var err error

	started = time.Now()
	result, err = Exec(t.TempDir()).Execute(context.Background(), `{"command":"sleep 60 & sleep 5","timeout_seconds":1}`)
	elapsed = time.Since(started)

	if err == nil {
		t.Fatalf("expected a timeout error, got result %q", result)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want a timeout", err)
	}
	if elapsed > 15*time.Second {
		t.Fatalf("bash_exec took %v to return, the backgrounded child held the pipe open", elapsed)
	}
}

func TestExecReturnsOutputFromAFailedCommand(t *testing.T) {
	var result string

	var err error

	result, err = Exec(t.TempDir()).Execute(context.Background(), `{"command":"printf stdout; printf stderr >&2; exit 1"}`)
	if err == nil || !strings.Contains(err.Error(), "command failed") {
		t.Fatalf("error = %v, want a command failure", err)
	}
	if !strings.Contains(result, "stdout") || !strings.Contains(result, "stderr") {
		t.Fatalf("result = %q, want stdout and stderr", result)
	}
}

func TestExecLimitsOutputWhileTheCommandRuns(t *testing.T) {
	var result string

	var err error

	result, err = Exec(t.TempDir()).Execute(context.Background(), `{"command":"yes x | head -c 70000"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) > maxOutput {
		t.Fatalf("result length = %d, want at most %d", len(result), maxOutput)
	}
	if !strings.HasSuffix(result, truncatedOutput) {
		t.Fatalf("result does not end with %q", truncatedOutput)
	}
}

func TestExecPreservesContextCancellation(t *testing.T) {
	var ctx context.Context
	var cancel context.CancelFunc
	var result chan error
	var started time.Time
	var elapsed time.Duration

	var err error

	ctx, cancel = context.WithCancel(context.Background())
	result = make(chan error, 1)
	started = time.Now()
	go func() {
		var executeErr error

		_, executeErr = Exec(t.TempDir()).Execute(ctx, `{"command":"sleep 60"}`)
		result <- executeErr
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	err = <-result
	elapsed = time.Since(started)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("bash_exec took %v to return after cancellation", elapsed)
	}
}

func TestShellResolvesToAnAbsolutePath(t *testing.T) {
	var binary string

	var err error

	binary, err = shell()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(binary, "/") {
		t.Fatalf("shell returned %q, want an absolute path", binary)
	}
}

func TestShellHonorsOverride(t *testing.T) {
	var binary string

	var err error

	t.Setenv("MININARU_SHELL", "sh")

	binary, err = shell()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(binary, "/sh") {
		t.Fatalf("shell returned %q, want the overridden sh", binary)
	}
}

func TestExecRejectsMissingCommand(t *testing.T) {
	var err error

	_, err = Exec(t.TempDir()).Execute(context.Background(), `{}`)
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("error = %v, want a missing command error", err)
	}
}
