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

func setupTestSessionSendRoundtrip(t *testing.T) *httptest.Server {
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
				"\"function\":{\"name\":\"session_send\",\"arguments\":\"{\\\"session\\\":\\\"s2\\\",\\\"content\\\":\\\"ping\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		case 2:
			fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"content\":\"pong\"},\"finish_reason\":null}]}\n\n")
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

func TestSessionSendDeliversAndReturnsTheReply(t *testing.T) {
	var caller *Agent
	var session *Session
	var target *Session
	var history []*Message
	var toolEvents []string

	var err error

	setupTestDB(t)
	setupTestSessionSendRoundtrip(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}
	err = AgentCreate(caller)
	if err != nil {
		t.Fatal(err)
	}

	session = &Session{Id: "s1", AgentId: "a1"}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}

	target = &Session{Id: "s2", AgentId: "a1"}
	err = SessionCreate(target)
	if err != nil {
		t.Fatal(err)
	}

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "ask the other session"})
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), caller, session, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {},
		func(name, status, message string) { toolEvents = append(toolEvents, name+":"+status) },
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	if !strings.Contains(strings.Join(toolEvents, ","), "session_send:started") ||
		!strings.Contains(strings.Join(toolEvents, ","), "session_send:finished") {
		t.Fatalf("tool events = %v, want a started/finished pair for the session_send call", toolEvents)
	}

	history, err = MessageList("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[1].Role != "assistant" || history[1].Content != "all done" {
		t.Fatalf("caller history = %+v, want a final assistant message %q", history, "all done")
	}

	history, err = MessageList("s2")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Content != "ping" || history[1].Content != "pong" {
		t.Fatalf("target session history = %+v", history)
	}
}

func TestSessionSendRefusesItsOwnSession(t *testing.T) {
	var caller *Agent
	var tool modules.Tool

	var err error

	setupTestDB(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}
	err = AgentCreate(caller)
	if err != nil {
		t.Fatal(err)
	}

	tool = sessionSendTool(caller, "s1", t.TempDir(), 0, nil, nil)

	_, err = tool.Execute(t.Context(), `{"session":"s1","content":"hi"}`)
	if err == nil || !strings.Contains(err.Error(), "own session") {
		t.Fatalf("error = %v, want a self-target refusal", err)
	}
}

func TestResolveSessionRefFindsByIdThenByName(t *testing.T) {
	var target *Session
	var found *Session

	var err error

	setupTestDB(t)

	err = AgentCreate(&Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	target = &Session{Id: "s2", AgentId: "a1", Name: "quiet-otter"}
	err = SessionCreate(target)
	if err != nil {
		t.Fatal(err)
	}

	found, err = resolveSessionRef("a1", "s2")
	if err != nil || found.Id != "s2" {
		t.Fatalf("resolve by id: got %+v, err %v", found, err)
	}

	found, err = resolveSessionRef("a1", "quiet-otter")
	if err != nil || found.Id != "s2" {
		t.Fatalf("resolve by name: got %+v, err %v", found, err)
	}

	_, err = resolveSessionRef("a1", "does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "no session") {
		t.Fatalf("resolve unknown: err = %v, want a friendly no-session message", err)
	}
}

func TestSessionSendRefusesADifferentAgentsSession(t *testing.T) {
	var caller *Agent
	var other *Agent
	var target *Session
	var tool modules.Tool

	var err error

	setupTestDB(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}
	err = AgentCreate(caller)
	if err != nil {
		t.Fatal(err)
	}

	other = &Agent{Id: "a2", Name: "other", Model: "gpt-4o-mini"}
	err = AgentCreate(other)
	if err != nil {
		t.Fatal(err)
	}

	target = &Session{Id: "s2", AgentId: "a2"}
	err = SessionCreate(target)
	if err != nil {
		t.Fatal(err)
	}

	tool = sessionSendTool(caller, "s1", t.TempDir(), 0, nil, nil)

	_, err = tool.Execute(t.Context(), `{"session":"s2","content":"hi"}`)
	if err == nil || !strings.Contains(err.Error(), "not owned by agent") {
		t.Fatalf("error = %v, want an ownership refusal", err)
	}
}

func TestBuildToolsHidesSessionSendBeyondMaxDepth(t *testing.T) {
	var caller *Agent
	var tools []modules.Tool
	var tool modules.Tool
	var found bool

	setupTestDB(t)

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}

	tools = buildTools(t.TempDir(), "s1", caller, 0, nil, nil)
	for _, tool = range tools {
		if tool.Name == SessionSendToolName {
			found = true
		}
	}
	if !found {
		t.Fatal("session_send missing from the top-level tool list")
	}

	found = false
	tools = buildTools(t.TempDir(), "s1", caller, maxSpawnDepth, nil, nil)
	for _, tool = range tools {
		if tool.Name == SessionSendToolName {
			found = true
		}
	}
	if found {
		t.Fatal("session_send should not be offered once the spawn depth limit is reached")
	}
}

func TestSessionSendMirrorsTheInjectedMessageBeforeTheReply(t *testing.T) {
	var caller *Agent
	var session *Session
	var target *Session
	var events []string

	var err error

	setupTestDB(t)
	setupTestSessionSendRoundtrip(t)

	SetSessionRouter(func(sessionId, origin, content string) {
		events = append(events, "message:"+sessionId+":"+origin+":"+content)
	}, func(sessionId string, chunk openai.ChatCompletionChunk) {
		events = append(events, "chunk:"+sessionId)
	}, func(sessionId, name, status, message string) {
		events = append(events, "tool:"+sessionId+":"+name+":"+status)
	}, func(sessionId, failure string) {
		events = append(events, "done:"+sessionId+":"+failure)
	})
	t.Cleanup(func() {
		SetSessionRouter(nil, nil, nil, nil)
	})

	caller = &Agent{Id: "a1", Name: "caller", Model: "gpt-4o-mini"}
	err = AgentCreate(caller)
	if err != nil {
		t.Fatal(err)
	}

	session = &Session{Id: "s1", AgentId: "a1"}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}

	target = &Session{Id: "s2", AgentId: "a1"}
	err = SessionCreate(target)
	if err != nil {
		t.Fatal(err)
	}

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "ask the other session"})
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), caller, session, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {},
		func(name, status, message string) {},
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	if len(events) == 0 || events[0] != "message:s2:s1:ping" {
		t.Fatalf("events = %v, want the injected message mirrored first", events)
	}

	if !strings.Contains(strings.Join(events, ","), "chunk:s2") {
		t.Fatalf("events = %v, want the target's reply chunks mirrored", events)
	}

	if events[len(events)-1] != "done:s2:" {
		t.Fatalf("events = %v, want a terminal done mirrored last", events)
	}
}
