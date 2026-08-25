// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestIsCommandDetectsSlashPrefix(t *testing.T) {
	var cases map[string]bool
	var line string
	var want bool

	cases = map[string]bool{"/help": true, "  /reset": true, "hello": false, "": false}

	for line, want = range cases {
		if isCommand(line) != want {
			t.Fatalf("isCommand(%q) = %v, want %v", line, isCommand(line), want)
		}
	}
}

func TestHelpCommandListsRegisteredCommands(t *testing.T) {
	var result commandResult

	var err error

	result, err = helpCommand(&state{}, nil)
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	if !strings.Contains(result.Message, "/reset") {
		t.Fatalf("help output missing /reset: %q", result.Message)
	}
}

func TestLeaveAgentCommandSetsExit(t *testing.T) {
	var result commandResult

	var err error

	result, err = leaveAgentCommand(&state{}, nil)
	if err != nil {
		t.Fatalf("exit failed: %v", err)
	}

	if !result.Exit {
		t.Fatal("/exit should set commandResult.Exit")
	}
}

func TestQuitShellCommandSetsQuit(t *testing.T) {
	var result commandResult

	var err error

	result, err = quitShellCommand(&state{}, nil)
	if err != nil {
		t.Fatalf("quit failed: %v", err)
	}

	if !result.Quit {
		t.Fatal("/exit should set commandResult.Quit")
	}
}

func TestDispatchCommandExitReturnsEOF(t *testing.T) {
	var sh state
	var err error

	sh = state{mode: MODE_AGENT}

	err = dispatchCommand(&sh, "/exit")
	if !errors.Is(err, io.EOF) {
		t.Fatalf("dispatchCommand(/exit) = %v, want io.EOF", err)
	}
}

func TestInfoCommandOfflineShowsOfflineStatus(t *testing.T) {
	var sh state
	var result commandResult
	var output string

	var err error

	sh = state{}

	output = captureStdout(t, func() {
		result, err = infoCommand(&sh, nil)
	})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}
	if result.Message != "" {
		t.Fatalf("offline info should not also return a session message: %q", result.Message)
	}
	if !strings.Contains(output, "offline") {
		t.Fatalf("info output missing offline status: %q", output)
	}
}

func TestClearScreenCommandSetsFlag(t *testing.T) {
	var result commandResult

	var err error

	result, err = clearScreenCommand(&state{}, nil)
	if err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if !result.ClearScreen {
		t.Fatal("/clear should set commandResult.ClearScreen")
	}
}

func TestYoloCommandRequiresAnArgument(t *testing.T) {
	var err error

	_, err = yoloCommand(&state{}, nil)
	if err == nil {
		t.Fatal("/yolo with no arguments should fail")
	}
}

func TestYoloCommandRejectsUnknownMode(t *testing.T) {
	var err error

	_, err = yoloCommand(&state{}, []string{"maybe"})
	if err == nil {
		t.Fatal("/yolo maybe should be rejected")
	}
}

func TestCommandNamesIncludesBuiltins(t *testing.T) {
	var names []string
	var name string
	var found bool

	names = commandNames()

	for _, name = range names {
		if name == "reset" {
			found = true
		}
	}

	if !found {
		t.Fatalf("commandNames() = %v, missing %q", names, "reset")
	}
}
