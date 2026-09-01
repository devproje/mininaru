// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"

	"github.com/devproje/mininaru/modules"
)

func hasFilesystemTools(tools []modules.Tool) bool {
	var tool modules.Tool
	var name string

	for _, tool = range tools {
		for _, name = range []string{"bash_exec", "file_read", "file_write", "file_edit"} {
			if tool.Name == name {
				return true
			}
		}
	}

	return false
}

func TestBuildToolsModeByRoot(t *testing.T) {
	var caller *Agent

	setupTestDB(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}

	if !hasFilesystemTools(buildTools(t.TempDir(), "s1", caller, 0, nil, nil)) {
		t.Fatal("dev mode is missing the filesystem tools")
	}

	if hasFilesystemTools(buildTools("", "s1", caller, 0, nil, nil)) {
		t.Fatal("chat mode still exposes the filesystem tools")
	}
}
