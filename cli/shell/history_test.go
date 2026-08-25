// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"os"
	"testing"
)

func TestRecordHistoryAppendsInOrder(t *testing.T) {
	var sh state
	var lines []string
	var line string
	var i int

	sh = state{}
	lines = []string{"ls -al", "cd /tmp", "history"}

	for _, line = range lines {
		recordHistory(&sh, line)
	}

	if len(sh.history) != len(lines) {
		t.Fatalf("got %d history entries, want %d", len(sh.history), len(lines))
	}

	for i, line = range lines {
		if sh.history[i] != line {
			t.Fatalf("entry %d: got %q, want %q", i, sh.history[i], line)
		}
	}
}

func TestRecordHistoryHonorsHistSize(t *testing.T) {
	var sh state
	var i int

	os.Setenv("HISTSIZE", "3")
	defer os.Unsetenv("HISTSIZE")

	sh = state{}
	for i = 0; i < 5; i++ {
		recordHistory(&sh, string(rune('a'+i)))
	}

	if len(sh.history) != 3 {
		t.Fatalf("got %d history entries, want 3", len(sh.history))
	}

	if sh.history[0] != "c" || sh.history[2] != "e" {
		t.Fatalf("unexpected trimmed history: %v", sh.history)
	}
}

func TestRecordHistoryKeepsAgentAndBashSeparate(t *testing.T) {
	var sh state

	sh = state{mode: MODE_BASH}
	recordHistory(&sh, "ls -al")

	sh.mode = MODE_AGENT
	recordHistory(&sh, "hello agent")

	if len(sh.history) != 1 || sh.history[0] != "ls -al" {
		t.Fatalf("bash history = %v, want [ls -al]", sh.history)
	}

	if len(sh.agentHistory) != 1 || sh.agentHistory[0] != "hello agent" {
		t.Fatalf("agent history = %v, want [hello agent]", sh.agentHistory)
	}
}

func TestHistoryForSelectsByMode(t *testing.T) {
	var sh state

	sh = state{mode: MODE_BASH, history: []string{"bash1"}, agentHistory: []string{"agent1"}}

	if len(historyFor(&sh)) != 1 || historyFor(&sh)[0] != "bash1" {
		t.Fatalf("historyFor(bash) = %v, want [bash1]", historyFor(&sh))
	}

	sh.mode = MODE_AGENT
	if len(historyFor(&sh)) != 1 || historyFor(&sh)[0] != "agent1" {
		t.Fatalf("historyFor(agent) = %v, want [agent1]", historyFor(&sh))
	}
}

func TestTrimHistoryKeepsMostRecent(t *testing.T) {
	var got []string

	got = trimHistory([]string{"a", "b", "c", "d"}, 2)
	if len(got) != 2 || got[0] != "c" || got[1] != "d" {
		t.Fatalf("got %v, want [c d]", got)
	}

	got = trimHistory([]string{"a", "b"}, 0)
	if len(got) != 2 {
		t.Fatalf("limit <= 0 should not trim, got %v", got)
	}
}

func TestDeleteHistoryEntrySupportsPositiveAndNegativeOffsets(t *testing.T) {
	var sh state

	sh = state{history: []string{"a", "b", "c", "d"}}

	if !deleteHistoryEntry(&sh, 2) {
		t.Fatalf("expected offset 2 to delete successfully")
	}

	if len(sh.history) != 3 || sh.history[1] != "c" {
		t.Fatalf("unexpected history after positive delete: %v", sh.history)
	}

	if !deleteHistoryEntry(&sh, -1) {
		t.Fatalf("expected offset -1 to delete successfully")
	}

	if len(sh.history) != 2 || sh.history[len(sh.history)-1] != "c" {
		t.Fatalf("unexpected history after negative delete: %v", sh.history)
	}

	if deleteHistoryEntry(&sh, 99) {
		t.Fatalf("out-of-range offset should not delete anything")
	}
}

func TestHistFileHonorsEnvOverride(t *testing.T) {
	os.Setenv("HISTFILE", "/tmp/mininaru-shell-history-test")
	defer os.Unsetenv("HISTFILE")

	if histFile() != "/tmp/mininaru-shell-history-test" {
		t.Fatalf("got %q, want env override to win", histFile())
	}
}
