// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package sock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gorilla/websocket"
)

func setupQuestionFixture(t *testing.T) (string, string) {
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
				"\"function\":{\"name\":\"ask_user_question\",\"arguments\":\"{\\\"question\\\":\\\"favorite color?\\\",\\\"options\\\":[\\\"red\\\",\\\"blue\\\"]}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()

			return
		}

		fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
			"\"delta\":{\"content\":\"got it\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	t.Cleanup(upstream.Close)

	err = core.ProviderCreate(&core.Provider{Id: "p1", Name: "test", BaseUrl: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}

	err = core.AgentCreate(&core.Agent{Id: "a1", Name: "naru", Model: "test:gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	err = core.SessionCreate(&core.Session{Id: "s1", AgentId: "a1"})
	if err != nil {
		t.Fatal(err)
	}

	return "s1", anchor
}

func TestSockHandlerAnswersQuestionThenFinishesTheTurn(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame
	var gotChunk bool
	var calls []*core.ToolCall
	var msgs []*core.Message

	var err error

	setupTestDB(t)
	sessionId, anchor = setupQuestionFixture(t)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "ask me something", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "question_request" {
		t.Fatalf("frame = %+v, want a question_request", frame)
	}
	if frame.Question != "favorite color?" {
		t.Fatalf("question = %q, want %q", frame.Question, "favorite color?")
	}
	if len(frame.Options) != 2 || frame.Options[0] != "red" || frame.Options[1] != "blue" {
		t.Fatalf("options = %v, want [red blue]", frame.Options)
	}
	if frame.SessionId != sessionId {
		t.Fatalf("question session = %q, want %q", frame.SessionId, sessionId)
	}

	err = conn.WriteJSON(map[string]string{"type": "question", "session_id": sessionId, "answer": "blue"})
	if err != nil {
		t.Fatal(err)
	}

	gotChunk, frame = readUntilTerminal(t, conn)
	if !gotChunk || frame.Type != "done" {
		t.Fatalf("turn did not complete after the answer: gotChunk=%v frame=%+v", gotChunk, frame)
	}

	msgs, err = core.MessageList(sessionId)
	if err != nil {
		t.Fatal(err)
	}
	calls, err = core.ToolCallList(msgs[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].Status != "completed" || !strings.Contains(calls[0].Result, "blue") {
		t.Fatalf("tool calls = %+v, want one completed ask_user_question call returning blue", calls)
	}
}
