// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestPlatformPromptFallsBackToDefaultWhenAbsent(t *testing.T) {
	var prompt string

	var err error

	setupTestFS(t)

	prompt, err = PlatformPrompt()
	if err != nil {
		t.Fatal(err)
	}
	if prompt != defaultPlatformPrompt {
		t.Fatalf("prompt = %q, want the built-in default", prompt)
	}
}

func TestPlatformPromptReadsCustomInstructionWhenPresent(t *testing.T) {
	var prompt string

	var err error

	setupTestFS(t)

	err = os.WriteFile(util.Path(customInstructionPath), []byte("custom rules here"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	prompt, err = PlatformPrompt()
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "custom rules here" {
		t.Fatalf("prompt = %q, want the custom instruction file's content", prompt)
	}
}
