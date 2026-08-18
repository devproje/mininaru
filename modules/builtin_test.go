// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type builtinContextTestKey struct{}

func TestBuiltinServerExposesEveryTool(t *testing.T) {
	var expected map[string]bool
	var result *mcp.ListToolsResult
	var listed *mcp.Tool

	var err error

	expected = map[string]bool{
		"current_time": true, "file_read": true, "file_write": true, "file_edit": true, "glob": true,
		"grep": true, "bash_exec": true, "web_search": true, "skill": true, "skill_create": true,
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

func TestBuiltinSessionKeepsConcurrentCallContextsSeparate(t *testing.T) {
	var tool builtinTool
	var server *mcp.Server
	var serverTransport *mcp.InMemoryTransport
	var clientTransport *mcp.InMemoryTransport
	var client *mcp.Client
	var session *mcp.ClientSession
	var execute func(context.Context, string) (string, error)
	var errors chan error
	var index int
	var expected string
	var wait sync.WaitGroup

	var err error

	tool = builtinTool{Build: func() Def {
		return Def{Execute: func(ctx context.Context, arguments string) (string, error) {
			var value string

			value, _ = ctx.Value(builtinContextTestKey{}).(string)
			return value, nil
		}}
	}}

	server = mcp.NewServer(&mcp.Implementation{Name: "context-test", Version: "test"}, nil)
	server.AddTool(&mcp.Tool{Name: "context", InputSchema: map[string]any{"type": "object"}}, builtinHandler(tool))

	serverTransport, clientTransport = mcp.NewInMemoryTransports()
	_, err = server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	client = mcp.NewClient(&mcp.Implementation{Name: "context-test", Version: "test"}, nil)
	session, err = client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })

	execute = builtinSessionExecute(session, "context")
	errors = make(chan error, 16)

	for index = 0; index < 16; index++ {
		expected = fmt.Sprintf("call-%d", index)
		wait.Add(1)

		go func(want string) {
			var ctx context.Context
			var output string

			var callErr error

			defer wait.Done()

			ctx = context.WithValue(context.Background(), builtinContextTestKey{}, want)
			output, callErr = execute(ctx, `{}`)
			if callErr != nil {
				errors <- callErr
				return
			}
			if output != want {
				errors <- fmt.Errorf("context output = %q, want %q", output, want)
			}
		}(expected)
	}

	wait.Wait()
	close(errors)

	for err = range errors {
		t.Error(err)
	}
}

func TestRegisterBuiltinExtendsTheBuiltinTable(t *testing.T) {
	var previousRegistered []builtinTool
	var previousCache []Def
	var tool builtinTool
	var found bool
	var permission Permission
	var annotations mcp.ToolAnnotations

	previousRegistered = registered
	previousCache = builtinCache

	t.Cleanup(func() {
		registered = previousRegistered
		builtinCache = previousCache
	})

	builtinCache = nil
	registered = nil

	RegisterBuiltin(func() Def {
		return Def{Name: "probe", Description: "probe", Permission: PermissionPrivileged,
			Parameters: map[string]any{"type": "object"}}
	}, BuiltinHints{Title: "probe", OpenWorld: true})

	for _, tool = range builtinTools() {
		if tool.Build().Name != "probe" {
			continue
		}

		found = true
		permission = tool.Permission
		annotations = tool.Annotations
	}

	if !found {
		t.Fatal("a registered tool did not reach the builtin table")
	}
	if permission != PermissionPrivileged {
		t.Fatalf("registered permission = %v, it should come from the def", permission)
	}
	if annotations.Title != "probe" || annotations.OpenWorldHint == nil || !*annotations.OpenWorldHint {
		t.Fatalf("registered annotations = %#v", annotations)
	}
}

func TestRegisterBuiltinRefusesAfterTheServerStarted(t *testing.T) {
	var previousRegistered []builtinTool
	var previousCache []Def

	previousRegistered = registered
	previousCache = builtinCache

	t.Cleanup(func() {
		registered = previousRegistered
		builtinCache = previousCache
	})

	builtinCache = []Def{{Name: "already"}}
	registered = nil

	RegisterBuiltin(func() Def { return Def{Name: "late"} }, BuiltinHints{Title: "late"})

	if len(registered) != 0 {
		t.Fatal("a tool registered after the builtin server had already started")
	}
}
