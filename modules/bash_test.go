// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBashExecUsesRoot(t *testing.T) {
	var root string
	var result string
	var err error

	root = t.TempDir()
	result, err = BashExec(root).Execute(context.Background(), `{"command":"pwd"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != filepath.Clean(root) {
		t.Fatalf("pwd = %q, want %q", result, root)
	}
}

func TestBashExecKillsBackgroundedChildOnTimeout(t *testing.T) {
	var started time.Time
	var elapsed time.Duration
	var result string

	var err error

	started = time.Now()
	result, err = BashExec(t.TempDir()).Execute(context.Background(),
		`{"command":"sleep 60 & sleep 5","timeout_seconds":1}`)
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

func TestBashShellResolvesToAnAbsolutePath(t *testing.T) {
	var shell string

	var err error

	shell, err = bashShell()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(shell, "/") {
		t.Fatalf("bashShell returned %q, want an absolute path", shell)
	}
}

func TestBashShellHonorsOverride(t *testing.T) {
	var shell string

	var err error

	t.Setenv("MININARU_SHELL", "sh")

	shell, err = bashShell()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(shell, "/sh") {
		t.Fatalf("bashShell returned %q, want the overridden sh", shell)
	}
}
