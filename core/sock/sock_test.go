// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package sock

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devproje/mininaru/modules/store"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type chatRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func readCompletion(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	var frame responseFrame
	var content strings.Builder
	var chunks int
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		if err := conn.ReadJSON(&frame); err != nil {
			t.Fatal(err)
		}
		switch frame.Type {
		case "chunk":
			chunks++
			content.WriteString(frame.Content)
		case "done":
			if chunks != 2 {
				t.Fatalf("expected two streamed chunks, got %d", chunks)
			}
			return content.String()
		default:
			t.Fatalf("unexpected response frame: %+v", frame)
		}
	}
}

func TestHandlerKeepsHistoryOnlyForConnection(t *testing.T) {
	var oldRoot string
	var oldDB *sql.DB
	var db *sql.DB
	var providerServer *httptest.Server
	var socketServer *httptest.Server
	var app *gin.Engine
	var conn *websocket.Conn
	var nextConn *websocket.Conn
	var requests chan chatRequest
	var request chatRequest
	var count int32
	var name string
	var model string
	var soul string
	var reply string

	var err error

	oldRoot = util.RootDir
	oldDB = util.DB
	defer func() {
		util.RootDir = oldRoot
		util.DB = oldDB
	}()
	if err = util.InitFS(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	db, err = util.NewDatabase(filepath.Join(util.RootDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	util.DB = db

	requests = make(chan chatRequest, 3)
	providerServer = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		var body chatRequest
		var number int32

		var decodeErr error

		decodeErr = json.NewDecoder(req.Body).Decode(&body)
		if decodeErr != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		number = atomic.AddInt32(&count, 1)
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(writer, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":0,\"model\":\"echo\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"reply-\"},\"finish_reason\":null}]}\n\n")
		_, _ = fmt.Fprintf(writer, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":0,\"model\":\"echo\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%d\"},\"finish_reason\":null}]}\n\n", number)
		_, _ = fmt.Fprint(writer, "data: [DONE]\n\n")
	}))
	defer providerServer.Close()

	_, err = store.AddProvider(context.Background(), "local", "test-key", providerServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	name, model, soul = "Naru", "local:echo", "be concise"
	if err = store.UpsertProfile(&store.Profile{Name: &name, Model: &model, Soul: &soul}); err != nil {
		t.Fatal(err)
	}

	app = gin.New()
	app.GET("/ws", Handler)
	socketServer = httptest.NewServer(app)
	defer socketServer.Close()

	conn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(socketServer.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for _, input := range []string{"hello", "again"} {
		if err = conn.WriteMessage(websocket.TextMessage, []byte(input)); err != nil {
			t.Fatal(err)
		}
		reply = readCompletion(t, conn)
		if reply != fmt.Sprintf("reply-%d", atomic.LoadInt32(&count)) {
			t.Fatalf("unexpected reply: %q", reply)
		}
	}

	request = <-requests
	if request.Model != "echo" || !request.Stream || len(request.Messages) != 2 || request.Messages[0].Role != "system" || request.Messages[1].Content != "hello" {
		t.Fatalf("first request did not include profile and user message: %+v", request)
	}
	request = <-requests
	if len(request.Messages) != 4 || request.Messages[2].Role != "assistant" || request.Messages[2].Content != "reply-1" || request.Messages[3].Content != "again" {
		t.Fatalf("second request did not include conversation history: %+v", request)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	nextConn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(socketServer.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer nextConn.Close()
	if err = nextConn.WriteMessage(websocket.TextMessage, []byte("new conversation")); err != nil {
		t.Fatal(err)
	}
	reply = readCompletion(t, nextConn)
	if reply != "reply-3" {
		t.Fatalf("unexpected reply on new connection: %q", reply)
	}
	request = <-requests
	if len(request.Messages) != 2 || request.Messages[1].Content != "new conversation" {
		t.Fatalf("new connection reused earlier conversation: %+v", request)
	}
}
