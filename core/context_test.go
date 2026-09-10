// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go"
)

func TestSendChatMessageCompactsOldTurns(t *testing.T) {
	var upstream *httptest.Server
	var agent *Agent
	var session *Session
	var summary *Summary
	var round int
	var messages string

	var err error

	setupTestDB(t)

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		var flusher http.Flusher

		json.NewDecoder(r.Body).Decode(&body)
		round++
		if round < 3 {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"summary","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"the old work is complete"},"finish_reason":"stop"}]}`)
			return
		}

		messages = fmt.Sprint(body["messages"])
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher = w.(http.Flusher)
		fmt.Fprint(w, "data: {\"id\":\"reply\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"done\"},\"finish_reason\":null}]}\n\n")
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

	agent = &Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini", MaxContext: 100000}
	err = AgentCreate(agent)
	if err != nil {
		t.Fatal(err)
	}
	session = &Session{Id: "s1", AgentId: agent.Id}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}

	err = MessageCreate(&Message{Id: "m1", SessionId: session.Id, Role: "user", Content: strings.Repeat("old first ", 5000), Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m2", SessionId: session.Id, Role: "assistant", Content: "old reply", Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m3", SessionId: session.Id, Role: "user", Content: strings.Repeat("old second ", 5000), Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m4", SessionId: session.Id, Role: "assistant", Content: "old second reply", Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m5", SessionId: session.Id, Role: "user", Content: "recent request", Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m6", SessionId: session.Id, Role: "assistant", Content: "recent reply", Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m7", SessionId: session.Id, Role: "user", Content: "current request"})
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), agent, session, t.TempDir(), 0, func(openai.ChatCompletionChunk) {}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if round != 3 {
		t.Fatalf("provider requests = %d, want two summaries plus reply", round)
	}
	if !strings.Contains(messages, "the old work is complete") || !strings.Contains(messages, "recent request") || !strings.Contains(messages, "current request") {
		t.Fatalf("reply messages = %q, want summary and recent raw history", messages)
	}
	if strings.Contains(messages, "old first") || strings.Contains(messages, "old second") {
		t.Fatalf("reply messages retained compacted history: %q", messages)
	}

	summary, err = SummaryLoad(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	if summary == nil || summary.ThroughMessageId != "m4" {
		t.Fatalf("summary = %+v, want marker m4", summary)
	}
}

func TestSummaryDeletesWithSession(t *testing.T) {
	var agent *Agent
	var session *Session
	var summary *Summary

	var err error

	setupTestDB(t)

	agent = &Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"}
	err = AgentCreate(agent)
	if err != nil {
		t.Fatal(err)
	}
	session = &Session{Id: "s1", AgentId: agent.Id}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m1", SessionId: session.Id, Role: "user", Content: "hello", Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = SummarySave(&Summary{SessionId: session.Id, Content: "summary", ThroughMessageId: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	err = SessionDelete(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	summary, err = SummaryLoad(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	if summary != nil {
		t.Fatalf("summary = %+v, want cascade deletion", summary)
	}
}

func TestSummaryTranscriptIncludesToolCalls(t *testing.T) {
	var agent *Agent
	var session *Session
	var transcript string

	var err error

	setupTestDB(t)

	agent = &Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"}
	err = AgentCreate(agent)
	if err != nil {
		t.Fatal(err)
	}
	session = &Session{Id: "s1", AgentId: agent.Id}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m1", SessionId: session.Id, Role: "user", Content: "inspect the repository", Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = ToolCallCreate(&ToolCall{Id: "t1", MessageId: "m1", CallId: "call_1", Name: "bash_exec", Arguments: `{"command":"git status"}`, Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	err = ToolCallUpdate("t1", &ToolCall{Result: "clean"})
	if err != nil {
		t.Fatal(err)
	}

	transcript, err = summaryTranscript("", []*Message{{Id: "m1", Role: "user", Content: "inspect the repository"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(transcript, "bash_exec") || !strings.Contains(transcript, "git status") || !strings.Contains(transcript, "clean") {
		t.Fatalf("transcript = %q, want tool name, arguments, and result", transcript)
	}
}

func TestChatContextCheckRejectsAnOversizedRequest(t *testing.T) {
	var agent Agent
	var err error
	var limitErr *ContextLengthError

	agent = Agent{MaxContext: 1000}
	err = ChatContextCheck(&agent, []ChatMessage{{Role: "user", Content: strings.Repeat("x", 2000)}})
	if !errors.As(err, &limitErr) {
		t.Fatalf("error = %v, want ContextLengthError", err)
	}
}
