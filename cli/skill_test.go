// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
)

func skillCommandSetup(t *testing.T) {
	var root string
	var bundle string

	var err error

	t.Helper()

	root = t.TempDir()
	bundle = filepath.Join(root, "pr-review")

	err = os.MkdirAll(bundle, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(bundle, "SKILL.md"),
		[]byte("---\nname: pr-review\ndescription: Review a pull request.\n---\n\nread the tests first\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = modules.SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { modules.SkillInitAt(t.TempDir(), "") })
}

func TestSkillShowRejectsAnUnknownName(t *testing.T) {
	var err error

	skillCommandSetup(t)

	err = skillShowExecute(skillShowCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("an unknown skill name was accepted")
	}
	if !strings.Contains(err.Error(), "pr-review") {
		t.Fatalf("the error does not list the available skills: %v", err)
	}
}

func TestSkillShowPrintsWhatTheModelReceives(t *testing.T) {
	var result string

	var err error

	skillCommandSetup(t)

	result, err = modules.SkillResult("pr-review", "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "skill: pr-review") {
		t.Fatalf("the header is missing: %q", result)
	}
	if !strings.Contains(result, "read the tests first") {
		t.Fatalf("the body is missing: %q", result)
	}
}
