// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package bash

import (
	"context"
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
