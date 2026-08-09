// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"
)

func fakeSession(t *testing.T, input string) *bytes.Buffer {
	var out bytes.Buffer
	var previousIn = askIn
	var previousOut = askOut
	var previousInteractive = askInteractive

	t.Helper()

	askIn = strings.NewReader(input)
	askOut = &out
	askInteractive = func() bool { return true }

	t.Cleanup(func() {
		askIn = previousIn
		askOut = previousOut
		askInteractive = previousInteractive
	})

	return &out
}

func TestAskTextKeepsTheFallbackOnEmptyInput(t *testing.T) {
	var answer string

	var err error

	fakeSession(t, "\n")

	answer, err = askText("provider name", "local")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "local" {
		t.Fatalf("answer = %q, want the fallback", answer)
	}
}

func TestAskTextTrimsTheAnswer(t *testing.T) {
	var answer string

	var err error

	fakeSession(t, "  openrouter  \n")

	answer, err = askText("provider name", "local")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "openrouter" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestAskRequiredRepeatsUntilAnswered(t *testing.T) {
	var out *bytes.Buffer
	var answer string

	var err error

	out = fakeSession(t, "\n\nnaru\n")

	answer, err = askRequired("agent name")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "naru" {
		t.Fatalf("answer = %q", answer)
	}
	if strings.Count(out.String(), "is required") != 2 {
		t.Fatalf("expected two retries, got: %s", out.String())
	}
}

func TestAskChoiceAcceptsANumberOrTheValue(t *testing.T) {
	var answer string

	var err error

	fakeSession(t, "2\n")

	answer, err = askChoice("provider", []string{"local", "openrouter"}, "local")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "openrouter" {
		t.Fatalf("numeric pick = %q", answer)
	}

	fakeSession(t, "local\n")

	answer, err = askChoice("provider", []string{"local", "openrouter"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "local" {
		t.Fatalf("named pick = %q", answer)
	}
}

func TestAskChoiceRejectsAnOutOfRangeNumber(t *testing.T) {
	var out *bytes.Buffer
	var answer string

	var err error

	out = fakeSession(t, "9\n1\n")

	answer, err = askChoice("provider", []string{"local", "openrouter"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "local" {
		t.Fatalf("answer = %q", answer)
	}
	if !strings.Contains(out.String(), "pick a number from 1 to 2") {
		t.Fatalf("no retry hint: %s", out.String())
	}
}

func TestAskConfirmDefaults(t *testing.T) {
	var answer bool

	var err error

	fakeSession(t, "\n")

	answer, err = askConfirm("keep it", true)
	if err != nil || !answer {
		t.Fatalf("empty answer = %v, %v, want the true fallback", answer, err)
	}

	fakeSession(t, "\n")

	answer, err = askConfirm("keep it", false)
	if err != nil || answer {
		t.Fatalf("empty answer = %v, %v, want the false fallback", answer, err)
	}

	fakeSession(t, "y\n")

	answer, err = askConfirm("keep it", false)
	if err != nil || !answer {
		t.Fatalf("y = %v, %v", answer, err)
	}
}

func TestAskPromptsGoToTheGivenWriterNotStdout(t *testing.T) {
	var out *bytes.Buffer

	var err error

	out = fakeSession(t, "naru\n")

	_, err = askRequired("agent name")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "agent name") {
		t.Fatalf("prompt did not reach the writer: %q", out.String())
	}
}

func TestTerminalSessionIsFalseForAPipe(t *testing.T) {
	var previousIn = askIn
	var previousOut = askOut

	t.Cleanup(func() {
		askIn = previousIn
		askOut = previousOut
	})

	askIn = strings.NewReader("")
	askOut = &bytes.Buffer{}

	if terminalSession() {
		t.Fatal("a pipe was reported as a terminal, which would make scripted runs hang")
	}
}
