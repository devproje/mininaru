// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

type testFrame struct {
	Type      string `json:"type"`
	SessionId string `json:"session_id"`
	Chunk     *struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	} `json:"chunk,omitempty"`
	Message   string `json:"message,omitempty"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func setupTestDB(t *testing.T) {
	var err error

	t.Helper()

	gin.SetMode(gin.TestMode)

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.NewDatabase(util.Path("data.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		util.DB.Close()
	})
}

func setupChatFixture(t *testing.T) string {
	var upstream *httptest.Server

	var err error

	t.Helper()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var flusher http.Flusher

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher = w.(http.Flusher)

		for _, c := range []string{"Hel", "lo"} {
			fmt.Fprintf(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n", c)
			flusher.Flush()
		}
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

	return "s1"
}

func newTestConn(t *testing.T) *websocket.Conn {
	var router *gin.Engine
	var server *httptest.Server
	var wsURL string
	var conn *websocket.Conn

	var err error

	t.Helper()

	router = gin.New()
	router.GET("/ws", SockHandler)

	server = httptest.NewServer(router)
	t.Cleanup(server.Close)

	wsURL = "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

func readFrame(t *testing.T, conn *websocket.Conn) testFrame {
	var frame testFrame

	var err error

	t.Helper()

	err = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}

	err = conn.ReadJSON(&frame)
	if err != nil {
		t.Fatal(err)
	}

	return frame
}

func readUntilTerminal(t *testing.T, conn *websocket.Conn) (bool, testFrame) {
	var gotChunk bool

	t.Helper()

	for {
		frame := readFrame(t, conn)
		if frame.Type == "chunk" {
			gotChunk = true
			continue
		}
		if frame.Type == "tool" {
			continue
		}

		return gotChunk, frame
	}
}

func TestSockHandlerCompletesATurn(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var gotChunk bool
	var final testFrame
	var msgs []*core.Message

	var err error

	setupTestDB(t)
	sessionId = setupChatFixture(t)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "hi"})
	if err != nil {
		t.Fatal(err)
	}

	gotChunk, final = readUntilTerminal(t, conn)
	if !gotChunk {
		t.Fatal("expected at least one chunk frame")
	}
	if final.Type != "done" {
		t.Fatalf("final frame type = %q, want done (message=%q)", final.Type, final.Message)
	}

	msgs, err = core.MessageList(sessionId)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Status != "completed" {
		t.Fatalf("user message = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Hello" {
		t.Fatalf("assistant message = %+v", msgs[1])
	}
}

func TestSockHandlerUnknownSessionStaysOpen(t *testing.T) {
	var conn *websocket.Conn
	var sessionId string
	var frame testFrame
	var gotChunk bool

	var err error

	setupTestDB(t)
	sessionId = setupChatFixture(t)
	conn = newTestConn(t)

	err = conn.WriteJSON(map[string]string{"session_id": "missing", "content": "hi"})
	if err != nil {
		t.Fatal(err)
	}

	frame = readFrame(t, conn)
	if frame.Type != "error" {
		t.Fatalf("type = %q, want error", frame.Type)
	}
	if frame.Message != "session not found" {
		t.Fatalf("message = %q, want 'session not found'", frame.Message)
	}

	err = conn.WriteJSON(map[string]string{"session_id": sessionId, "content": "again"})
	if err != nil {
		t.Fatal(err)
	}

	gotChunk, frame = readUntilTerminal(t, conn)
	if !gotChunk || frame.Type != "done" {
		t.Fatalf("second turn did not complete: gotChunk=%v type=%q", gotChunk, frame.Type)
	}
}

func TestSockHandlerMalformedFrame(t *testing.T) {
	var conn *websocket.Conn
	var frame testFrame

	var err error

	setupTestDB(t)
	conn = newTestConn(t)

	err = conn.WriteMessage(websocket.TextMessage, []byte("not json"))
	if err != nil {
		t.Fatal(err)
	}

	frame = readFrame(t, conn)
	if frame.Type != "error" {
		t.Fatalf("type = %q, want error", frame.Type)
	}
}

func TestChunkReasoningReadsProviderFields(t *testing.T) {
	var chunk openai.ChatCompletionChunk

	var err error

	err = chunk.UnmarshalJSON([]byte(`{"choices":[{"delta":{"reasoning_content":"stepping through"}}]}`))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if chunkReasoning(chunk) != "stepping through" {
		t.Fatalf("reasoning_content was not read: %q", chunkReasoning(chunk))
	}

	err = chunk.UnmarshalJSON([]byte(`{"choices":[{"delta":{"reasoning":"another form"}}]}`))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if chunkReasoning(chunk) != "another form" {
		t.Fatalf("reasoning was not read: %q", chunkReasoning(chunk))
	}

	err = chunk.UnmarshalJSON([]byte(`{"choices":[{"delta":{"content":"plain"}}]}`))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if chunkReasoning(chunk) != "" {
		t.Fatalf("plain content should carry no reasoning")
	}
}
