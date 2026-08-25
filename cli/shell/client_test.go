// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func captureStdout(t *testing.T, run func()) string {
	var reader *os.File
	var writer *os.File
	var previous *os.File
	var buf []byte

	var err error

	reader, writer, err = os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	previous = os.Stdout
	os.Stdout = writer

	run()

	os.Stdout = previous
	writer.Close()

	buf, err = io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	return string(buf)
}

func TestSendAgentRendersReasoningBeforeAnswer(t *testing.T) {
	var upgrader websocket.Upgrader
	var server *httptest.Server
	var client *websocket.Conn
	var sh state
	var output string

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		var conn *websocket.Conn
		var raw []byte
		var frames []string
		var payload string

		var handlerErr error

		conn, handlerErr = upgrader.Upgrade(res, req, nil)
		if handlerErr != nil {
			return
		}
		defer conn.Close()

		_, raw, handlerErr = conn.ReadMessage()
		if handlerErr != nil {
			return
		}

		if !strings.Contains(string(raw), "why is the sky blue") {
			return
		}

		frames = []string{
			`{"type":"chunk","session_id":"s","reasoning":"weighing rayleigh scattering"}`,
			`{"type":"chunk","session_id":"s","chunk":{"choices":[{"delta":{"content":"short wavelengths scatter"}}]}}`,
			`{"type":"done","session_id":"s"}`,
		}

		for _, payload = range frames {
			handlerErr = conn.WriteMessage(websocket.TextMessage, []byte(payload))
			if handlerErr != nil {
				return
			}
		}
	}))
	defer server.Close()

	client, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	sh = state{conn: client, session: "s"}

	output = captureStdout(t, func() {
		err = sendAgent(&sh, "why is the sky blue")
	})
	if err != nil {
		t.Fatalf("sendAgent: %v", err)
	}

	if !strings.Contains(output, "◇ thinking") || !strings.Contains(output, "weighing rayleigh scattering") {
		t.Fatalf("reasoning was not rendered: %q", output)
	}

	if strings.Index(output, "weighing rayleigh scattering") > strings.Index(output, "◆ agent") {
		t.Fatalf("reasoning should be printed before the answer: %q", output)
	}

	if !strings.Contains(output, "short wavelengths scatter") {
		t.Fatalf("answer was not rendered: %q", output)
	}
}

func TestReplyDecodesReasoningField(t *testing.T) {
	var got reply

	var err error

	err = json.Unmarshal([]byte(`{"type":"chunk","reasoning":"hmm"}`), &got)
	if err != nil || got.Reasoning != "hmm" {
		t.Fatalf("got %+v, err %v", got, err)
	}
}
