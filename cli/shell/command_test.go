// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

func setupTestNaruPath(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
}

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

func newAgentTestServer(t *testing.T) (*httptest.Server, *string) {
	var server *httptest.Server
	var createdAgentId string

	t.Helper()

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any

		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/agents/worker":
			json.NewEncoder(w).Encode(map[string]string{"id": "a-worker", "name": "worker"})
		case r.URL.Path == "/api/agents/missing":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		case r.URL.Path == "/api/agents":
			json.NewEncoder(w).Encode([]map[string]string{{"id": "a-worker", "name": "worker"}, {"id": "a-naru", "name": "naru"}})
		case r.URL.Path == "/api/sessions/s1":
			json.NewEncoder(w).Encode(map[string]string{"id": "s1", "agent_id": "a-naru"})
		case r.URL.Path == "/api/sessions":
			json.NewDecoder(r.Body).Decode(&body)
			createdAgentId, _ = body["agent_id"].(string)
			json.NewEncoder(w).Encode(map[string]string{"id": "s2", "agent_id": createdAgentId})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	return server, &createdAgentId
}

func TestAgentCommandRequiresAnArgument(t *testing.T) {
	var err error

	_, err = agentCommand(&state{}, nil)
	if err == nil {
		t.Fatal("/agent with no arguments should fail")
	}
}

func TestAgentCommandSetsDefaultAgentByName(t *testing.T) {
	var server *httptest.Server
	var sh state
	var result commandResult

	var err error

	setupTestNaruPath(t)
	server, _ = newAgentTestServer(t)
	sh = state{url: server.URL}

	result, err = agentCommand(&sh, []string{"worker"})
	if err != nil {
		t.Fatalf("/agent worker failed: %v", err)
	}
	if sh.agent != "worker" {
		t.Fatalf("sh.agent = %q, want %q", sh.agent, "worker")
	}
	if !strings.Contains(result.Message, "worker") {
		t.Fatalf("result message = %q, want it to mention worker", result.Message)
	}
}

func TestAgentCommandPersistsTheDefaultAcrossShellRestarts(t *testing.T) {
	var server *httptest.Server
	var sh state
	var prefs *preferences

	var err error

	setupTestNaruPath(t)
	server, _ = newAgentTestServer(t)
	sh = state{url: server.URL}

	_, err = agentCommand(&sh, []string{"worker"})
	if err != nil {
		t.Fatalf("/agent worker failed: %v", err)
	}

	prefs, err = loadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if prefs.Agent != "worker" {
		t.Fatalf("persisted agent = %q, want %q", prefs.Agent, "worker")
	}
}

func TestAgentCommandRejectsUnknownAgent(t *testing.T) {
	var server *httptest.Server
	var sh state

	var err error

	server, _ = newAgentTestServer(t)
	sh = state{url: server.URL}

	_, err = agentCommand(&sh, []string{"missing"})
	if err == nil {
		t.Fatal("/agent missing should fail")
	}
}

func TestResolveAgentByIdOrNameMatchesById(t *testing.T) {
	var server *httptest.Server
	var sh state
	var base string
	var target *core.Agent

	var err error

	server, _ = newAgentTestServer(t)
	sh = state{url: server.URL}

	base, err = apiBase(sh.url)
	if err != nil {
		t.Fatal(err)
	}

	target, err = resolveAgentByIdOrName(&sh, base, "worker")
	if err != nil {
		t.Fatalf("resolveAgentByIdOrName by id failed: %v", err)
	}
	if target.Name != "worker" {
		t.Fatalf("target.Name = %q, want %q", target.Name, "worker")
	}
}

func TestResetSessionCommandUsesConfiguredAgent(t *testing.T) {
	var server *httptest.Server
	var createdAgentId *string
	var sh state

	var err error

	server, createdAgentId = newAgentTestServer(t)
	sh = state{url: server.URL, session: "s1", agent: "worker"}

	_, err = resetSessionCommand(&sh, nil)
	if err != nil {
		t.Fatalf("/reset failed: %v", err)
	}
	if *createdAgentId != "a-worker" {
		t.Fatalf("new session agent_id = %q, want the configured agent's id %q", *createdAgentId, "a-worker")
	}
}

func TestResetSessionCommandKeepsCurrentAgentWhenUnset(t *testing.T) {
	var server *httptest.Server
	var createdAgentId *string
	var sh state

	var err error

	server, createdAgentId = newAgentTestServer(t)
	sh = state{url: server.URL, session: "s1"}

	_, err = resetSessionCommand(&sh, nil)
	if err != nil {
		t.Fatalf("/reset failed: %v", err)
	}
	if *createdAgentId != "a-naru" {
		t.Fatalf("new session agent_id = %q, want the current session's agent id %q", *createdAgentId, "a-naru")
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
