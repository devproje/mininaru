// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuiltinServerExposesEveryTool(t *testing.T) {
	var expected map[string]bool
	var result *mcp.ListToolsResult
	var listed *mcp.Tool

	var err error

	expected = map[string]bool{
		"current_time": true, "file_read": true, "file_write": true, "file_edit": true,
		"bash_exec": true, "web_search": true, "skill": true, "skill_create": true,
		"web_fetch": true, "memory": true,
	}

	if len(builtinDefs()) == 0 {
		t.Fatal("builtin server did not connect")
	}

	result, err = builtinSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Tools) != len(expected) {
		t.Fatalf("builtin server listed %d tools", len(result.Tools))
	}

	for _, listed = range result.Tools {
		if !expected[listed.Name] {
			t.Fatalf("unexpected builtin tool %q", listed.Name)
		}
		if listed.Annotations == nil {
			t.Fatalf("builtin tool %q has no annotations", listed.Name)
		}
		if listed.InputSchema == nil {
			t.Fatalf("builtin tool %q has no input schema", listed.Name)
		}
	}
}

func TestBuiltinSandboxSurvivesSession(t *testing.T) {
	var root string
	var outside string
	var previous string
	var def *Def

	var err error

	root = t.TempDir()
	outside = t.TempDir()

	err = os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(outside, filepath.Join(root, "escape"))
	if err != nil {
		t.Fatal(err)
	}

	previous = workingRoot
	defer func() { workingRoot = previous }()

	err = SetWorkingRoot(root)
	if err != nil {
		t.Fatal(err)
	}

	def = findBuiltin(t, "file_read")

	_, err = def.Execute(context.Background(), `{"path":"escape/secret.txt"}`)
	if err == nil {
		t.Fatal("file_read through the mcp session escaped the working root")
	}
}

func TestBuiltinFailureCarriesOutput(t *testing.T) {
	var root string
	var previous string
	var def *Def
	var output string

	var err error

	root = t.TempDir()
	previous = workingRoot
	defer func() { workingRoot = previous }()

	err = SetWorkingRoot(root)
	if err != nil {
		t.Fatal(err)
	}

	def = findBuiltin(t, "bash_exec")

	output, err = def.Execute(context.Background(), `{"command":"echo partial; exit 3"}`)
	if err == nil {
		t.Fatalf("failing bash_exec returned no error: %q", output)
	}
	if !strings.Contains(err.Error(), "partial") {
		t.Fatalf("failing bash_exec dropped its output: %v", err)
	}
}

func findBuiltin(t *testing.T, name string) *Def {
	var defs []Def
	var index int

	t.Helper()

	defs = builtinDefs()

	for index = range defs {
		if defs[index].Name != name {
			continue
		}

		return &defs[index]
	}

	t.Fatalf("builtin tool %q not found", name)
	return nil
}
