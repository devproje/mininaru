// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestPromptReceiveApproval(t *testing.T) {
	var srv *httptest.Server
	var conn *websocket.Conn
	var got promptFrame
	var read, write *os.File
	var stdin *os.File
	var done chan struct{}

	var err error

	done = make(chan struct{})

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var up websocket.Upgrader
		var server *websocket.Conn
		var reply promptReply

		var err error

		defer close(done)

		server, err = up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer server.Close()

		for _, reply = range []promptReply{
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

		err = server.WriteJSON(promptReply{Type: "done", SessionId: "s1"})
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

	err = promptReceive(conn, "s1")
	if err != nil {
		t.Fatal(err)
	}

	<-done

	if got.Type != "approval" || got.Decision != "once" {
		t.Fatalf("want approval/once, got %q/%q", got.Type, got.Decision)
	}
}
