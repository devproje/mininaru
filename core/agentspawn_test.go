// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
)

func setupTestSpawnRoundtrip(t *testing.T) *httptest.Server {
	var upstream *httptest.Server
	var round int

	var err error

	t.Helper()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var flusher http.Flusher

		round++

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher = w.(http.Flusher)

		switch round {
		case 1:
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\","+
				"\"function\":{\"name\":\"agent_spawn\",\"arguments\":\"{\\\"agent\\\":\\\"worker\\\",\\\"prompt\\\":\\\"do the subtask\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		case 2:
			fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"content\":\"subtask done\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			fmt.Fprint(w, "data: {\"id\":\"c3\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"content\":\"all done\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
		flusher.Flush()
	}))
	t.Cleanup(upstream.Close)

	err = ProviderCreate(&Provider{Id: "p1", Name: "test", BaseUrl: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}
	err = ProviderActivate("p1")
	if err != nil {
		t.Fatal(err)
	}

	return upstream
}

func TestAgentSpawnDelegatesAndReturnsTheAnswer(t *testing.T) {
	var caller *Agent
	var worker *Agent
	var session *Session
	var sessions []*Session
	var spawned *Session
	var history []*Message
	var toolEvents []string

	var err error

	setupTestDB(t)
	setupTestSpawnRoundtrip(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}
	err = AgentCreate(caller)
	if err != nil {
		t.Fatal(err)
	}

	worker = &Agent{Id: "a2", Name: "worker", Model: "gpt-4o-mini"}
	err = AgentCreate(worker)
	if err != nil {
		t.Fatal(err)
	}

	session = &Session{Id: "s1", AgentId: "a1"}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "hand this off"})
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), caller, session, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {},
		func(name, status, message string) { toolEvents = append(toolEvents, name+":"+status) },
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	if !strings.Contains(strings.Join(toolEvents, ","), "worker:started") ||
		!strings.Contains(strings.Join(toolEvents, ","), "worker:finished") {
		t.Fatalf("tool events = %v, want a started/finished pair for the delegate", toolEvents)
	}

	history, err = MessageList("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[1].Role != "assistant" || history[1].Content != "all done" {
		t.Fatalf("caller history = %+v, want a final assistant message %q", history, "all done")
	}

	sessions, err = SessionList("a2")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("worker sessions = %d, want 1 (the spawned session)", len(sessions))
	}

	spawned = sessions[0]
	if !strings.Contains(spawned.Name, "do the subtask") {
		t.Fatalf("spawned session name = %q, want it to preview the prompt", spawned.Name)
	}

	history, err = MessageList(spawned.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Content != "do the subtask" || history[1].Content != "subtask done" {
		t.Fatalf("spawned session history = %+v", history)
	}
}

func TestAgentSpawnRefusesSelfDelegation(t *testing.T) {
	var caller *Agent
	var tool modules.Tool
	var err error

	setupTestDB(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}
	err = AgentCreate(caller)
	if err != nil {
		t.Fatal(err)
	}

	tool = agentSpawnTool(caller, t.TempDir(), 0, nil, nil)

	_, err = tool.Execute(t.Context(), `{"agent":"caller","prompt":"do it"}`)
	if err == nil || !strings.Contains(err.Error(), "cannot spawn itself") {
		t.Fatalf("error = %v, want a self-spawn refusal", err)
	}
}

func TestBuildToolsHidesAgentSpawnBeyondMaxDepth(t *testing.T) {
	var caller *Agent
	var tools []modules.Tool
	var tool modules.Tool
	var found bool

	setupTestDB(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}

	tools = buildTools(t.TempDir(), "s1", caller, 0, nil, nil)
	for _, tool = range tools {
		if tool.Name == AgentSpawnToolName {
			found = true
		}
	}
	if !found {
		t.Fatal("agent_spawn missing from the top-level tool list")
	}

	found = false
	tools = buildTools(t.TempDir(), "s1", caller, maxSpawnDepth, nil, nil)
	for _, tool = range tools {
		if tool.Name == AgentSpawnToolName {
			found = true
		}
	}
	if found {
		t.Fatal("agent_spawn should not be offered once the spawn depth limit is reached")
	}
}
