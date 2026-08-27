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
)

func setupTestToolRoundtrip(t *testing.T) *httptest.Server {
	var upstream *httptest.Server
	var round int

	var err error

	t.Helper()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		var flusher http.Flusher

		json.NewDecoder(r.Body).Decode(&body)
		round++

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher = w.(http.Flusher)

		if round == 1 {
			if body["tools"] == nil {
				t.Errorf("first request has no tools field")
			}

			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\","+
				"\"function\":{\"name\":\"bash_exec\",\"arguments\":\"{\\\"command\\\":\\\"echo hi\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()

			return
		}

		if !strings.Contains(fmt.Sprint(body["messages"]), "hi") {
			t.Errorf("second request messages missing the tool result: %v", body["messages"])
		}

		fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
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

	return upstream
}

func TestSendChatMessageToolRoundtrip(t *testing.T) {
	var agent *Agent
	var session *Session
	var toolCalls []*ToolCall
	var history []*Message

	var err error

	setupTestDB(t)
	setupTestToolRoundtrip(t)

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

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "run echo hi"})
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), agent, session, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {}, func(name, status, message string) {},
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	toolCalls, err = ToolCallList("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(toolCalls))
	}
	if toolCalls[0].Status != "completed" || !strings.Contains(toolCalls[0].Result, "hi") {
		t.Fatalf("tool call = %+v, want a completed bash_exec result containing hi", toolCalls[0])
	}

	history, err = MessageList("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history = %d messages, want 2 (user + assistant)", len(history))
	}
	if history[1].Role != "assistant" || history[1].Content != "done" {
		t.Fatalf("assistant message = %+v, want content %q", history[1], "done")
	}
}

func setupTestDenyRoundtrip(t *testing.T) *httptest.Server {
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

		if round == 1 {
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\","+
				"\"function\":{\"name\":\"bash_exec\",\"arguments\":\"{\\\"command\\\":\\\"echo hi\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()

			return
		}

		fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
			"\"delta\":{\"content\":\"acknowledged the denial\"},\"finish_reason\":null}]}\n\n")
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

	return upstream
}

func TestSendChatMessageDeniesDangerousToolOnDeny(t *testing.T) {
	var agent *Agent
	var session *Session
	var toolCalls []*ToolCall
	var approveCalls int

	var err error

	setupTestDB(t)
	setupTestDenyRoundtrip(t)

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

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "run echo hi"})
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), agent, session, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {}, func(name, status, message string) {},
		func(ctx context.Context, name, arguments string) (string, error) {
			approveCalls++
			return "deny", nil
		})
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	if approveCalls != 1 {
		t.Fatalf("approve was called %d times, want 1", approveCalls)
	}

	toolCalls, err = ToolCallList("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 1 || toolCalls[0].Status != "failed" || !strings.Contains(toolCalls[0].Error, "denied") {
		t.Fatalf("tool call = %+v, want a failed call with a denial error", toolCalls)
	}
}
