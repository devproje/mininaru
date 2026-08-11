// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
)

func toolChunk(id, delta, finish string) string {
	return `data: {"id":"` + id + `","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":` + delta + `,"finish_reason":` + finish + `}]}` + "\n\n"
}

func TestChatExecutesToolAndReturnsFinalAnswer(t *testing.T) {
	var srv *httptest.Server
	var requests []string
	var session *Session
	var agent *NaruAgent
	var def modules.Def
	var executions int
	var payload struct {
		Text string `json:"text"`
	}
	var message *Message
	var events []ToolEvent
	var history []*Message
	var calls []*ToolCall

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "text/event-stream")

		if len(requests) == 1 {
			io.WriteString(w, toolChunk("round-1", `{"role":"assistant","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"echo","arguments":"{\"text\":\"hello\"}"}}]}`, `"tool_calls"`))
		} else {
			io.WriteString(w, toolChunk("round-2", `{"role":"assistant","content":"tool said hello"}`, `"stop"`))
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = thinkingSetup(t, srv.URL)
	def = modules.Def{
		Name: "echo", Description: "echo text", Permission: modules.PermissionSafe,
		Parameters: map[string]any{"type": "object"},
		Execute: func(ctx context.Context, arguments string) (string, error) {
			executions++
			err = json.Unmarshal([]byte(arguments), &payload)
			return payload.Text, err
		},
	}

	message, err = chatWithTools(context.Background(), session, agent, "use echo", []modules.Def{def}, nil, nil,
		func(event ToolEvent) { events = append(events, event) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if message.Content != "tool said hello" || executions != 1 || len(requests) != 2 {
		t.Fatalf("message=%q executions=%d requests=%d", message.Content, executions, len(requests))
	}
	if !strings.Contains(requests[0], `"tools"`) || !strings.Contains(requests[1], `"role":"tool"`) || !strings.Contains(requests[1], `"hello"`) {
		t.Fatalf("tool protocol missing from requests: %#v", requests)
	}

	history, err = MessageList(session.Id)
	if err != nil {
		t.Fatal(err)
	}
	calls, err = ToolCallList(history[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].CallId != "call-1" || calls[0].Status != MessageCompleted {
		t.Fatalf("tool call audit = %#v", calls)
	}
	if len(events) != 2 || events[0].Phase != ToolEventStarted || events[1].Phase != ToolEventFinished || events[1].Result != "hello" {
		t.Fatalf("tool events = %#v", events)
	}
}

func TestToolDefinitionsIncludeDangerousToolsForApproval(t *testing.T) {
	var defs []modules.Def

	defs = permittedTools([]modules.Def{
		{Name: "safe", Permission: modules.PermissionSafe, Execute: func(context.Context, string) (string, error) { return "", nil }},
		{Name: "danger", Permission: modules.PermissionDangerous, Execute: func(context.Context, string) (string, error) { return "", nil }},
	})
	if len(defs) != 2 {
		t.Fatalf("tool definitions = %#v", defs)
	}
}

func TestDangerousToolRequiresApprovalOrBypass(t *testing.T) {
	var session *Session
	var pending *Message
	var defs []modules.Def
	var executions int
	var call openai.ChatCompletionMessageToolCall
	var record *ToolCall
	var calls []*ToolCall
	var approvals int
	var result string
	var approved bool

	var err error

	session, _ = thinkingSetup(t, "http://127.0.0.1")
	pending, err = messageStart(session.Id, "danger")
	if err != nil {
		t.Fatal(err)
	}
	defs = []modules.Def{{
		Name: "danger", Permission: modules.PermissionDangerous,
		Execute: func(context.Context, string) (string, error) {
			executions++
			return "executed", nil
		},
	}}
	call = openai.ChatCompletionMessageToolCall{ID: "call-denied"}
	call.Function.Name = "danger"
	call.Function.Arguments = `{}`

	record, err = toolCallStart(pending.Id, call)
	if err != nil {
		t.Fatal(err)
	}
	calls, err = ToolCallList(pending.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].Status != MessagePending {
		t.Fatalf("started tool call = %#v", calls)
	}
	record, err = executeTool(context.Background(), "", record, defs, false, true,
		func(context.Context, modules.Def, string) (bool, error) {
			approvals++
			return false, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	result = record.Result
	if !strings.Contains(result, "user denied") || executions != 0 || approvals != 1 {
		t.Fatalf("denied result=%q executions=%d approvals=%d", result, executions, approvals)
	}

	call.ID = "call-approved"
	record, err = toolCallStart(pending.Id, call)
	if err != nil {
		t.Fatal(err)
	}
	record, err = executeTool(context.Background(), "", record, defs, false, true,
		func(context.Context, modules.Def, string) (bool, error) {
			approvals++
			return true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	result = record.Result
	if result != "executed" || executions != 1 || approvals != 2 {
		t.Fatalf("approved result=%q executions=%d approvals=%d", result, executions, approvals)
	}

	call.ID = "call-bypass"
	approved = false
	record, err = toolCallStart(pending.Id, call)
	if err != nil {
		t.Fatal(err)
	}
	record, err = executeTool(context.Background(), "", record, defs, true, true,
		func(context.Context, modules.Def, string) (bool, error) {
			approved = true
			return false, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	result = record.Result
	if result != "executed" || executions != 2 || approved {
		t.Fatalf("bypass result=%q executions=%d approval_called=%v", result, executions, approved)
	}
}

func TestResumedSessionReplaysToolCallsToTheModel(t *testing.T) {
	var srv *httptest.Server
	var requests []string
	var session *Session
	var agent *NaruAgent
	var def modules.Def

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "text/event-stream")

		if len(requests) == 1 {
			io.WriteString(w, toolChunk("r1", `{"role":"assistant","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"echo","arguments":"{\"text\":\"remembered\"}"}}]}`, `"tool_calls"`))
		} else {
			io.WriteString(w, toolChunk("r2", `{"role":"assistant","content":"ok"}`, `"stop"`))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = thinkingSetup(t, srv.URL)
	def = modules.Def{
		Name: "echo", Permission: modules.PermissionSafe, Parameters: map[string]any{"type": "object"},
		Execute: func(context.Context, string) (string, error) { return "remembered", nil },
	}

	_, err = chatWithTools(context.Background(), session, agent, "first", []modules.Def{def}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = chatWithTools(context.Background(), session, agent, "second", []modules.Def{def}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 3 {
		t.Fatalf("upstream requests = %d, want 3", len(requests))
	}

	if !strings.Contains(requests[2], `"tool_call_id":"call-1"`) || !strings.Contains(requests[2], "remembered") {
		t.Fatalf("resumed request lost the tool history: %s", requests[2])
	}

	if strings.Count(requests[2], `"role":"tool"`) != 1 {
		t.Fatalf("resumed request tool message count = %d, want 1: %s", strings.Count(requests[2], `"role":"tool"`), requests[2])
	}
}

func TestHistorySkipsUnfinishedToolCalls(t *testing.T) {
	var history []*Message
	var calls map[string][]*ToolCall
	var messages []openai.ChatCompletionMessageParamUnion

	history = []*Message{{Id: "u1", Role: "user", Content: "hi"}, {Role: "assistant", Content: "yo"}}
	calls = map[string][]*ToolCall{
		"u1": {{CallId: "c1", Status: MessagePending, Name: "echo", Arguments: "{}"}},
	}

	messages = historyMessages(history, calls)
	if len(messages) != 2 {
		t.Fatalf("history replayed %d messages, want 2 without the pending tool call", len(messages))
	}
}
