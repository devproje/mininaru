// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gorilla/websocket"
)

func setupToolFixture(t *testing.T) (string, string) {
	var upstream *httptest.Server
	var round int
	var anchor string

	var err error

	t.Helper()

	anchor = t.TempDir()

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
			"\"delta\":{\"content\":\"done\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	t.Cleanup(upstream.Close)

	err = core.ProviderCreate(&core.Provider{Id: "p1", Name: "test", BaseUrl: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}
	err = core.ProviderActivate("p1")
	if err != nil {
		t.Fatal(err)
	}

	err = core.AgentCreate(&core.Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	err = core.SessionCreate(&core.Session{Id: "s1", AgentId: "a1"})
	if err != nil {
		t.Fatal(err)
	}

	return "s1", anchor
}

func readUntilApproval(t *testing.T, conn *websocket.Conn) testFrame {
	t.Helper()

	for {
		frame := readFrame(t, conn)
		if frame.Type == "chunk" || frame.Type == "tool" {
			continue
		}

		return frame
	}
}

func TestSockHandlerApprovesOnceThenRunsTheTool(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame
	var gotChunk bool

	var err error

	setupTestDB(t)
	sessionId, anchor = setupToolFixture(t)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "run echo hi", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "approval_request" || frame.Name != "bash_exec" {
		t.Fatalf("frame = %+v, want an approval_request for bash_exec", frame)
	}

	err = conn.WriteJSON(map[string]string{"type": "approval", "session_id": sessionId, "decision": "once"})
	if err != nil {
		t.Fatal(err)
	}

	gotChunk, frame = readUntilTerminal(t, conn)
	if !gotChunk || frame.Type != "done" {
		t.Fatalf("turn did not complete after approval: gotChunk=%v frame=%+v", gotChunk, frame)
	}
}

func TestSockHandlerDeniedToolStillCompletesTheTurn(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame
	var gotChunk bool
	var messages []*core.Message
	var calls []*core.ToolCall

	var err error

	setupTestDB(t)
	sessionId, anchor = setupToolFixture(t)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "run echo hi", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "approval_request" {
		t.Fatalf("frame = %+v, want an approval_request", frame)
	}

	err = conn.WriteJSON(map[string]string{"type": "approval", "session_id": sessionId, "decision": "deny"})
	if err != nil {
		t.Fatal(err)
	}

	gotChunk, frame = readUntilTerminal(t, conn)
	if !gotChunk || frame.Type != "done" {
		t.Fatalf("denial should still let the model acknowledge and finish: gotChunk=%v frame=%+v", gotChunk, frame)
	}

	messages, err = core.MessageList(sessionId)
	if err != nil {
		t.Fatal(err)
	}
	calls, err = core.ToolCallList(messages[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].Status != "failed" {
		t.Fatalf("tool calls = %+v, want one failed (denied) call", calls)
	}
}

func TestSockHandlerSessionDecisionSkipsFurtherPrompts(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame
	var gotChunk bool

	var err error

	setupTestDB(t)
	sessionId, anchor = setupToolFixture(t)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "run echo hi", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "approval_request" {
		t.Fatalf("first turn frame = %+v, want an approval_request", frame)
	}

	err = conn.WriteJSON(map[string]string{"type": "approval", "session_id": sessionId, "decision": "session"})
	if err != nil {
		t.Fatal(err)
	}

	gotChunk, frame = readUntilTerminal(t, conn)
	if !gotChunk || frame.Type != "done" {
		t.Fatalf("first turn did not complete: gotChunk=%v frame=%+v", gotChunk, frame)
	}

	if !sessionApproved(sessionId) {
		t.Fatal("session decision was not remembered")
	}
}
