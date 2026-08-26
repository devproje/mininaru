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

func setupSessionSendFixture(t *testing.T) {
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
				"\"delta\":{\"content\":\"warm\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		case 2:
			fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\","+
				"\"function\":{\"name\":\"session_send\",\"arguments\":\"{\\\"session\\\":\\\"s2\\\",\\\"content\\\":\\\"ping\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		case 3:
			fmt.Fprint(w, "data: {\"id\":\"c3\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"content\":\"pong\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			fmt.Fprint(w, "data: {\"id\":\"c4\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"content\":\"all done\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
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

	err = core.SessionCreate(&core.Session{Id: "s2", AgentId: "a1"})
	if err != nil {
		t.Fatal(err)
	}
}

func setupAttachFixture(t *testing.T) {
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

	err = core.SessionCreate(&core.Session{Id: "s2", AgentId: "a1"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAttachRegistersLiveConnWithoutStartingAChatRound(t *testing.T) {
	var viewer *websocket.Conn
	var caller *websocket.Conn
	var gotChunk bool
	var final testFrame
	var frame testFrame

	var err error

	setupTestDB(t)
	setupAttachFixture(t)

	viewer = newTestConn(t)
	err = viewer.WriteJSON(map[string]string{"type": "attach", "session_id": "s2"})
	if err != nil {
		t.Fatal(err)
	}

	caller = newTestConn(t)
	err = caller.WriteJSON(map[string]string{"session_id": "s1", "content": "ask s2"})
	if err != nil {
		t.Fatal(err)
	}

	for {
		frame = readFrame(t, caller)
		if frame.Type == "chunk" {
			gotChunk = true
			continue
		}
		if frame.Type == "tool" {
			continue
		}
		if frame.Type == "approval_request" {
			err = caller.WriteJSON(map[string]string{"type": "approval", "session_id": "s1", "decision": "once"})
			if err != nil {
				t.Fatal(err)
			}
			continue
		}

		final = frame
		break
	}
	if !gotChunk || final.Type != "done" {
		t.Fatalf("caller round did not complete: gotChunk=%v type=%q", gotChunk, final.Type)
	}

	for {
		frame = readFrame(t, viewer)
		if frame.Type != "chunk" {
			t.Fatalf("frame type = %q, want a mirrored chunk", frame.Type)
		}
		if frame.Chunk != nil && len(frame.Chunk.Choices) > 0 && frame.Chunk.Choices[0].Delta.Content == "pong" {
			break
		}
	}
}

func TestAttachToAnUnknownSessionReturnsAnError(t *testing.T) {
	var conn *websocket.Conn
	var frame testFrame

	var err error

	setupTestDB(t)

	conn = newTestConn(t)
	err = conn.WriteJSON(map[string]string{"type": "attach", "session_id": "missing"})
	if err != nil {
		t.Fatal(err)
	}

	frame = readFrame(t, conn)
	if frame.Type != "error" {
		t.Fatalf("type = %q, want error", frame.Type)
	}
}

func TestSessionSendMirrorsToALiveConnectedViewer(t *testing.T) {
	var viewer *websocket.Conn
	var caller *websocket.Conn
	var gotChunk bool
	var final testFrame
	var frame testFrame

	var err error

	setupTestDB(t)
	setupSessionSendFixture(t)

	viewer = newTestConn(t)
	err = viewer.WriteJSON(map[string]string{"session_id": "s2", "content": "hello viewer"})
	if err != nil {
		t.Fatal(err)
	}

	gotChunk, final = readUntilTerminal(t, viewer)
	if !gotChunk || final.Type != "done" {
		t.Fatalf("viewer warm-up round did not complete: gotChunk=%v type=%q", gotChunk, final.Type)
	}

	caller = newTestConn(t)
	err = caller.WriteJSON(map[string]string{"session_id": "s1", "content": "ask s2"})
	if err != nil {
		t.Fatal(err)
	}

	for {
		frame = readFrame(t, caller)
		if frame.Type == "chunk" {
			gotChunk = true
			continue
		}
		if frame.Type == "tool" {
			continue
		}
		if frame.Type == "approval_request" {
			err = caller.WriteJSON(map[string]string{"type": "approval", "session_id": "s1", "decision": "once"})
			if err != nil {
				t.Fatal(err)
			}
			continue
		}

		final = frame
		break
	}
	if !gotChunk || final.Type != "done" {
		t.Fatalf("caller round did not complete: gotChunk=%v type=%q", gotChunk, final.Type)
	}

	for {
		frame = readFrame(t, viewer)
		if frame.Type != "chunk" {
			t.Fatalf("frame type = %q, want a mirrored chunk", frame.Type)
		}
		if frame.Chunk != nil && len(frame.Chunk.Choices) > 0 && frame.Chunk.Choices[0].Delta.Content == "pong" {
			break
		}
	}
}
