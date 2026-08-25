// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
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
