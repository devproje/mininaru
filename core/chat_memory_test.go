// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go"

	"github.com/devproje/mininaru/modules/memory"
)

func setupTestMemoryChat(t *testing.T) *string {
	var upstream *httptest.Server
	var captured string

	var err error

	t.Helper()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		var flusher http.Flusher
		var encoded []byte

		json.NewDecoder(r.Body).Decode(&body)
		encoded, _ = json.Marshal(body["messages"])
		captured = string(encoded)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher = w.(http.Flusher)

		fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
			"\"delta\":{\"content\":\"done\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
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

	return &captured
}

func TestSendChatMessageIncludesSavedMemoryForItsAgent(t *testing.T) {
	var agent *Agent
	var session *Session
	var captured *string

	var err error

	setupTestDB(t)
	captured = setupTestMemoryChat(t)

	agent = &Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"}
	err = AgentCreate(agent)
	if err != nil {
		t.Fatal(err)
	}

	session = &Session{Id: "s1", AgentId: "a1"}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = memory.Tools("a1")[0].Execute(context.Background(),
		`{"name":"favorite-editor","description":"prefers vim","type":"user","content":"the user prefers vim"}`)
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), agent, session, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {}, func(name, status, message string) {},
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	if !strings.Contains(*captured, "favorite-editor.md") {
		t.Fatalf("request messages = %s, want it to include the memory index", *captured)
	}
}

func TestSendChatMessageDoesNotLeakMemoryAcrossAgents(t *testing.T) {
	var agentA *Agent
	var agentB *Agent
	var sessionB *Session
	var captured *string

	var err error

	setupTestDB(t)
	captured = setupTestMemoryChat(t)

	agentA = &Agent{Id: "a1", Name: "naru-a", Model: "gpt-4o-mini"}
	err = AgentCreate(agentA)
	if err != nil {
		t.Fatal(err)
	}

	agentB = &Agent{Id: "a2", Name: "naru-b", Model: "gpt-4o-mini"}
	err = AgentCreate(agentB)
	if err != nil {
		t.Fatal(err)
	}

	sessionB = &Session{Id: "s2", AgentId: "a2"}
	err = SessionCreate(sessionB)
	if err != nil {
		t.Fatal(err)
	}

	err = MessageCreate(&Message{Id: "m2", SessionId: "s2", Role: "user", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = memory.Tools("a1")[0].Execute(context.Background(),
		`{"name":"a-only","description":"belongs to agent a","type":"user","content":"secret to agent a"}`)
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), agentB, sessionB, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {}, func(name, status, message string) {},
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	if strings.Contains(*captured, "a-only.md") {
		t.Fatalf("request messages = %s, agent b should not see agent a's memory", *captured)
	}
}
