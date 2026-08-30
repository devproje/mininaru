// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

func chunkContent(text string) *openai.ChatCompletionChunk {
	var chunk openai.ChatCompletionChunk

	chunk.Choices = []openai.ChatCompletionChunkChoice{{Delta: openai.ChatCompletionChunkChoiceDelta{Content: text}}}

	return &chunk
}

func TestReceiveApproval(t *testing.T) {
	var srv *httptest.Server
	var conn *websocket.Conn
	var got Frame
	var read, write *os.File
	var stdin *os.File
	var done chan struct{}

	var err error

	done = make(chan struct{})

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var up websocket.Upgrader
		var server *websocket.Conn
		var reply Reply

		var err error

		defer close(done)

		server, err = up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer server.Close()

		for _, reply = range []Reply{
			{Type: "chunk", SessionId: "s1", Reasoning: "thinking"},
			{Type: "approval_request", SessionId: "s1", Name: "bash", Arguments: `{"cmd":"ls"}`},
		} {
			err = server.WriteJSON(reply)
			if err != nil {
				t.Error(err)
				return
			}
		}

		err = server.ReadJSON(&got)
		if err != nil {
			t.Error(err)
			return
		}

		err = server.WriteJSON(Reply{Type: "done", SessionId: "s1"})
		if err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	read, write, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	stdin = os.Stdin
	os.Stdin = read
	defer func() { os.Stdin = stdin }()

	_, err = write.WriteString("y\n")
	if err != nil {
		t.Fatal(err)
	}

	conn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	err = Receive(conn, Pump(conn), "s1", nil, "")
	if err != nil {
		t.Fatal(err)
	}

	<-done

	if got.Type != "approval" || got.Decision != "once" {
		t.Fatalf("want approval/once, got %q/%q", got.Type, got.Decision)
	}
}

func TestReceiveInterruptOnCtrlC(t *testing.T) {
	var srv *httptest.Server
	var conn *websocket.Conn
	var stream keys
	var got Frame
	var done chan struct{}

	var err error

	done = make(chan struct{})

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var up websocket.Upgrader
		var server *websocket.Conn

		var err error

		defer close(done)

		server, err = up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer server.Close()

		err = server.WriteJSON(Reply{Type: "chunk", SessionId: "s1", Reasoning: "working"})
		if err != nil {
			t.Error(err)
			return
		}

		err = server.ReadJSON(&got)
		if err != nil {
			t.Error(err)
			return
		}

		err = server.WriteJSON(Reply{Type: "done", SessionId: "s1"})
		if err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	conn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	stream = make(keys, 4)
	stream <- 0x03

	err = Receive(conn, Pump(conn), "s1", stream, "")
	if err != nil {
		t.Fatal(err)
	}

	<-done

	if got.Type != "interrupt" || got.SessionId != "s1" {
		t.Fatalf("want interrupt/s1, got %q/%q", got.Type, got.SessionId)
	}
}

func TestReceiveJSONFormat(t *testing.T) {
	var srv *httptest.Server
	var conn *websocket.Conn
	var result Result
	var read, write, stdout *os.File
	var captured []byte
	var done chan struct{}

	var err error

	done = make(chan struct{})

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var up websocket.Upgrader
		var server *websocket.Conn
		var reply Reply

		var err error

		defer close(done)

		server, err = up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer server.Close()

		for _, reply = range []Reply{
			{Type: "chunk", SessionId: "s1", Reasoning: "pondering"},
			{Type: "tool", SessionId: "s1", Name: "bash", Status: "started"},
			{Type: "tool", SessionId: "s1", Name: "bash", Status: "finished", Message: "ok"},
			{Type: "chunk", SessionId: "s1", Chunk: chunkContent("hi ")},
			{Type: "chunk", SessionId: "s1", Chunk: chunkContent("there")},
			{Type: "done", SessionId: "s1"},
		} {
			err = server.WriteJSON(reply)
			if err != nil {
				t.Error(err)
				return
			}
		}
	}))
	defer srv.Close()

	read, write, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout = os.Stdout
	os.Stdout = write

	conn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		os.Stdout = stdout
		t.Fatal(err)
	}
	defer conn.Close()

	err = Receive(conn, Pump(conn), "s1", nil, FormatJSON)
	write.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatal(err)
	}

	captured, err = io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(captured, &result)
	if err != nil {
		t.Fatalf("stdout is not one JSON object: %q", captured)
	}

	if result.Content != "hi there" {
		t.Fatalf("content = %q", result.Content)
	}

	if len(result.Tools) != 1 || result.Tools[0].Name != "bash" || result.Tools[0].Status != "finished" {
		t.Fatalf("tools = %+v", result.Tools)
	}

	<-done
}
