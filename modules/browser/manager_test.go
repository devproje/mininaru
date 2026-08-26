// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChromePathHonorsEnvOverride(t *testing.T) {
	t.Setenv("MININARU_CHROME", "/opt/custom/chrome")

	if chromePath() != "/opt/custom/chrome" {
		t.Fatalf("chromePath() = %q, want the env override", chromePath())
	}
	if !Available() {
		t.Fatal("Available() should be true once MININARU_CHROME is set, regardless of whether that path exists")
	}
}

func TestChromePathFallsBackToKnownAbsolutePaths(t *testing.T) {
	var dir string
	var fake string
	var original []string

	var err error

	dir = t.TempDir()
	fake = filepath.Join(dir, "headless_shell")

	err = os.WriteFile(fake, []byte("#!/bin/sh\n"), 0755)
	if err != nil {
		t.Fatal(err)
	}

	original = chromeAbsolutePaths
	chromeAbsolutePaths = []string{fake}
	t.Cleanup(func() { chromeAbsolutePaths = original })

	if chromePath() != fake {
		t.Fatalf("chromePath() = %q, want the fake path %q (not found on $PATH)", chromePath(), fake)
	}
}

func TestSessionContextReusesTheSameSession(t *testing.T) {
	var first *session
	var again *session
	var ok bool

	t.Cleanup(func() { closeSession("test-reuse") })

	sessionContext("test-reuse")

	mu.Lock()
	first, ok = sessions["test-reuse"]
	mu.Unlock()
	if !ok {
		t.Fatal("sessionContext did not register a session")
	}

	sessionContext("test-reuse")

	mu.Lock()
	again = sessions["test-reuse"]
	mu.Unlock()

	if first != again {
		t.Fatal("sessionContext created a second session for the same session id")
	}
}

func TestNewSessionDoesNotHangForeverOnAStalledLaunch(t *testing.T) {
	var dir string
	var fake string
	var previousTimeout time.Duration
	var start time.Time
	var elapsed time.Duration
	var ctx context.Context
	var err error

	dir = t.TempDir()
	fake = filepath.Join(dir, "fake-chrome")

	err = os.WriteFile(fake, []byte("#!/bin/sh\nsleep 100\n"), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("MININARU_CHROME", fake)

	previousTimeout = callTimeout
	callTimeout = 300 * time.Millisecond
	t.Cleanup(func() { callTimeout = previousTimeout })

	t.Cleanup(func() { closeSession("test-stall") })

	start = time.Now()
	ctx, err = sessionContext("test-stall")
	elapsed = time.Since(start)
	_ = ctx

	if err == nil {
		t.Fatal("expected the stalled Chrome launch to time out with an error")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("sessionContext took %v to fail, want it bounded by the (lowered) launch timeout", elapsed)
	}
}

func TestCloseSessionRemovesIt(t *testing.T) {
	var ok bool

	sessionContext("test-close")
	closeSession("test-close")

	mu.Lock()
	_, ok = sessions["test-close"]
	mu.Unlock()

	if ok {
		t.Fatal("closeSession left the session registered")
	}
}
