// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package sock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gorilla/websocket"
)

func setupPlanModeSingleToolFixture(t *testing.T, toolName, toolArguments string) (string, string) {
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
				"\"function\":{\"name\":\""+toolName+"\",\"arguments\":\""+toolArguments+"\"}}]},\"finish_reason\":null}]}\n\n")
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

func TestSockHandlerPlanModeAsksForReadOnlyTool(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame
	var gotChunk bool

	var err error

	setupTestDB(t)
	sessionId, anchor = setupPlanModeSingleToolFixture(t, "file_read", `{\"path\":\"notes.txt\"}`)

	err = os.WriteFile(filepath.Join(anchor, "notes.txt"), []byte("hello"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "read notes.txt", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "approval_request" || frame.Name != "file_read" {
		t.Fatalf("frame = %+v, want Plan mode to ask for a read-only tool instead of auto-denying it", frame)
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

func TestSockHandlerPlanModeStillAutoDeniesMutatingTool(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var anchor string
	var frame testFrame
	var calls []*core.ToolCall
	var msgs []*core.Message

	var err error

	setupTestDB(t)
	sessionId, anchor = setupPlanModeSingleToolFixture(t, "bash_exec", `{\"command\":\"echo hi\"}`)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "run echo hi", "cwd": anchor})
	if err != nil {
		t.Fatal(err)
	}

	frame = readUntilApproval(t, conn)
	if frame.Type != "done" {
		t.Fatalf("frame = %+v, want the turn to finish without ever asking for bash_exec in Plan mode", frame)
	}

	msgs, err = core.MessageList(sessionId)
	if err != nil {
		t.Fatal(err)
	}
	calls, err = core.ToolCallList(msgs[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].Status != "failed" || !strings.Contains(calls[0].Error, "not approved") {
		t.Fatalf("tool call = %+v, want bash_exec auto-denied without a prompt", calls)
	}
}
