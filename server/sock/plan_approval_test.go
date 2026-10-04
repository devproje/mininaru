// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package sock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gorilla/websocket"
)

func setupPlanApprovalFixture(t *testing.T, secondCall string) (string, string) {
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
				"\"function\":{\"name\":\"plan_approval\",\"arguments\":\"{\\\"plan\\\":\\\"write notes.txt\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()

			return
		}

		if round == 2 {
			fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_2\",\"type\":\"function\","+
				"\"function\":{\"name\":\"file_write\",\"arguments\":\""+secondCall+"\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()

			return
		}

		fmt.Fprint(w, "data: {\"id\":\"c3\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
			"\"delta\":{\"content\":\"done\"},\"finish_reason\":null}]}\n\n")
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

	err = core.ModeUpsert(anchor, core.ModePlan)
	if err != nil {
		t.Fatal(err)
	}

	return "s1", anchor
}

func TestSockHandlerPlanApprovalAutoPersistThenAutoRunsFileWrite(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame
	var gotChunk bool

	var err error

	setupTestDB(t)
	sessionId, anchor = setupPlanApprovalFixture(t, `{\"path\":\"notes.txt\",\"content\":\"hi\"}`)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "write a file", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "question_request" {
		t.Fatalf("frame = %+v, want a question_request from plan_approval", frame)
	}
	if frame.Question != "write notes.txt" {
		t.Fatalf("question = %q, want the plan text", frame.Question)
	}

	err = conn.WriteJSON(map[string]string{"type": "question", "session_id": sessionId, "answer": "Allow session (Persist)"})
	if err != nil {
		t.Fatal(err)
	}

	frame = readFrame(t, conn)
	if frame.Type != "tool" || frame.Status != "mode" || frame.Name != core.PlanApprovalToolName || frame.Message != core.ModeAutoPersist {
		t.Fatalf("frame = %+v, want a live mode-change frame for %s", frame, core.ModeAutoPersist)
	}

	gotChunk, frame = readUntilTerminal(t, conn)
	if !gotChunk || frame.Type != "done" {
		t.Fatalf("turn did not complete after choosing Allow session (Persist): gotChunk=%v frame=%+v", gotChunk, frame)
	}
}

func TestSockHandlerPlanApprovalDenyInterruptsTheTurn(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame

	var err error

	setupTestDB(t)
	sessionId, anchor = setupPlanApprovalFixture(t, `{\"path\":\"notes.txt\",\"content\":\"hi\"}`)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "write a file", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "question_request" {
		t.Fatalf("frame = %+v, want a question_request from plan_approval", frame)
	}

	err = conn.WriteJSON(map[string]string{"type": "question", "session_id": sessionId, "answer": "Deny"})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "error" {
		t.Fatalf("frame = %+v, want the turn to abort with an error after Deny", frame)
	}

	if core.ModeLookup(anchor) != core.ModePlan {
		t.Fatalf("mode = %q, want Plan unchanged after a deny", core.ModeLookup(anchor))
	}
}
