// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
)

func TestMain(m *testing.M) {
	InstallAgentTool()

	os.Exit(m.Run())
}

func subagentSetup(t *testing.T, srvURL string) (*Session, *NaruAgent, *NaruAgent) {
	var session *Session
	var parent *NaruAgent
	var worker *NaruAgent

	t.Helper()

	session, parent = thinkingSetup(t, srvURL)

	worker = AgentNew("worker", "you are the worker", "", "worker-model", Providers[0])

	Global = parent
	Agents = []*NaruAgent{worker}

	t.Cleanup(func() {
		Global = nil
		Agents = nil
	})

	return session, parent, worker
}

func delegationCall(agent, prompt string) string {
	return `{"role":"assistant","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"agent_call","arguments":"{\"agent\":\"` +
		agent + `\",\"prompt\":\"` + prompt + `\"}"}}]}`
}

func TestAgentCallDelegatesAndFeedsTheAnswerBack(t *testing.T) {
	var srv *httptest.Server
	var requests []string
	var session *Session
	var parent *NaruAgent
	var message *Message

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "text/event-stream")

		switch len(requests) {
		case 1:
			io.WriteString(w, toolChunk("r1", delegationCall("worker", "summarise the diff"), `"tool_calls"`))
		case 2:
			io.WriteString(w, toolChunk("r2", `{"role":"assistant","content":"the worker answer"}`, `"stop"`))
		default:
			io.WriteString(w, toolChunk("r3", `{"role":"assistant","content":"final"}`, `"stop"`))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, parent, _ = subagentSetup(t, srv.URL)

	message, err = ChatWithTools(context.Background(), session, parent, "go",
		[]modules.Def{AgentCallTool()}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if message.Content != "final" {
		t.Fatalf("parent answer = %q", message.Content)
	}
	if len(requests) != 3 {
		t.Fatalf("request count = %d, want 3", len(requests))
	}

	if !strings.Contains(requests[1], "worker-model") || !strings.Contains(requests[1], "you are the worker") {
		t.Fatalf("the subagent did not run as the worker: %s", requests[1])
	}
	if !strings.Contains(requests[1], "summarise the diff") {
		t.Fatalf("the subagent did not receive the prompt: %s", requests[1])
	}
	if !strings.Contains(requests[2], "the worker answer") {
		t.Fatalf("the answer was not fed back to the parent: %s", requests[2])
	}
}

func TestAgentCallIsNotOfferedToTheSubagent(t *testing.T) {
	var srv *httptest.Server
	var requests []string
	var session *Session
	var parent *NaruAgent
	var defs []modules.Def

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "text/event-stream")

		switch len(requests) {
		case 1:
			io.WriteString(w, toolChunk("r1", delegationCall("worker", "work"), `"tool_calls"`))
		case 2:
			io.WriteString(w, toolChunk("r2", `{"role":"assistant","content":"done"}`, `"stop"`))
		default:
			io.WriteString(w, toolChunk("r3", `{"role":"assistant","content":"final"}`, `"stop"`))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, parent, _ = subagentSetup(t, srv.URL)

	defs = []modules.Def{
		AgentCallTool(),
		{Name: "shell", Description: "shell", Permission: modules.PermissionDangerous,
			Parameters: map[string]any{"type": "object"},
			Execute:    func(context.Context, string) (string, error) { return "ran", nil }},
	}

	_, err = ChatWithTools(context.Background(), session, parent, "go", defs, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(requests[1], AgentToolName) {
		t.Fatalf("the subagent was offered %s and could recurse: %s", AgentToolName, requests[1])
	}
	if !strings.Contains(requests[1], `"shell"`) {
		t.Fatalf("the subagent did not inherit the parent's other tools: %s", requests[1])
	}
}

func TestAgentCallRefusesSelfDelegationAndUnknownAgents(t *testing.T) {
	var srv *httptest.Server
	var session *Session
	var parent *NaruAgent
	var ctx context.Context

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	session, parent, _ = subagentSetup(t, srv.URL)
	_ = session

	ctx = subagentContext(context.Background(), subagentPolicy{CallerId: parent.Id, AllowPrivileged: true})

	_, err = AgentCallTool().Execute(ctx, `{"agent":"naru","prompt":"do it"}`)
	if err == nil || !strings.Contains(err.Error(), "cannot delegate to itself") {
		t.Fatalf("self delegation error = %v", err)
	}

	_, err = AgentCallTool().Execute(ctx, `{"agent":"nobody","prompt":"do it"}`)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown agent error = %v", err)
	}
}

func TestAgentCallRefusesBeyondTheDepthLimit(t *testing.T) {
	var srv *httptest.Server
	var parent *NaruAgent
	var ctx context.Context

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_, parent, _ = subagentSetup(t, srv.URL)

	ctx = subagentContext(context.Background(), subagentPolicy{CallerId: parent.Id, Depth: maxSubagentDepth})

	_, err = AgentCallTool().Execute(ctx, `{"agent":"worker","prompt":"go deeper"}`)
	if err == nil || !strings.Contains(err.Error(), "limited to") {
		t.Fatalf("depth limit error = %v", err)
	}
}

func TestAgentCallRefusedOutsideAChatTurn(t *testing.T) {
	var err error

	_, err = AgentCallTool().Execute(context.Background(), `{"agent":"worker","prompt":"go"}`)
	if err == nil || !strings.Contains(err.Error(), "inside a chat turn") {
		t.Fatalf("out of turn error = %v", err)
	}
}

func TestAgentCallValidatesArguments(t *testing.T) {
	var err error

	_, err = AgentCallTool().Execute(context.Background(), `{"prompt":"go"}`)
	if err == nil || !strings.Contains(err.Error(), "agent is required") {
		t.Fatalf("missing agent error = %v", err)
	}

	_, err = AgentCallTool().Execute(context.Background(), `{"agent":"worker"}`)
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("missing prompt error = %v", err)
	}
}

func TestAgentCallIsPrivileged(t *testing.T) {
	if AgentCallTool().Permission != modules.PermissionPrivileged {
		t.Fatalf("%s permission = %v", AgentToolName, AgentCallTool().Permission)
	}
}

func TestInstallAgentToolReachesTheModelThroughMCP(t *testing.T) {
	var def modules.Def
	var found bool
	var permission modules.Permission

	InstallAgentTool()

	for _, def = range modules.DefaultTools() {
		if def.Name != AgentToolName {
			continue
		}

		found = true
		permission = def.Permission
	}

	if !found {
		t.Fatalf("%s did not reach the tool list through the builtin mcp server", AgentToolName)
	}
	if permission != modules.PermissionPrivileged {
		t.Fatalf("%s permission through mcp = %v", AgentToolName, permission)
	}

	for _, def = range modules.SafeTools() {
		if def.Name == AgentToolName {
			t.Fatalf("%s was offered to a daemon front end", AgentToolName)
		}
	}
}
