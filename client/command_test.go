// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"strings"
	"testing"
)

func TestDispatchUnknown(t *testing.T) {
	var sh Shell

	var err error

	err = dispatch(&sh, "/nope arg")
	if err == nil || !strings.Contains(err.Error(), "/nope") {
		t.Fatalf("want unknown command error, got %v", err)
	}
}

func TestDispatchExit(t *testing.T) {
	var sh Shell

	var err error

	err = dispatch(&sh, "/exit")
	if err != nil {
		t.Fatal(err)
	}

	if !sh.quit {
		t.Fatal("/exit did not set quit")
	}
}

func TestEffortRejectsUnknownLevel(t *testing.T) {
	var sh Shell

	var err error

	err = dispatch(&sh, "/effort turbo")
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("want usage error, got %v", err)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	var name string

	for name = range commands {
		if commands[name].run == nil {
			t.Fatalf("/%s has no handler", name)
		}
	}
}
