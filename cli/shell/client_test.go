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
	"time"

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

func TestIsReasoningFiller(t *testing.T) {
	var cases map[string]bool
	var s string
	var want bool

	cases = map[string]bool{
		".":                       true,
		"...":                     true,
		"  ...  ":                 true,
		"":                        false,
		"weighing rayleigh":       false,
		".The Google search":      false,
		"...thinking about it...": false,
	}

	for s, want = range cases {
		if isReasoningFiller(s) != want {
			t.Fatalf("isReasoningFiller(%q) = %v, want %v", s, isReasoningFiller(s), want)
		}
	}
}

func TestSendAgentSuppressesDotFillerReasoning(t *testing.T) {
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

		if !strings.Contains(string(raw), "weather") {
			return
		}

		frames = []string{
			`{"type":"chunk","session_id":"s","reasoning":"."}`,
			`{"type":"chunk","session_id":"s","reasoning":"."}`,
			`{"type":"chunk","session_id":"s","reasoning":"checking the forecast"}`,
			`{"type":"chunk","session_id":"s","chunk":{"choices":[{"delta":{"content":"it's sunny"}}]}}`,
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
		err = sendAgent(&sh, "weather")
	})
	if err != nil {
		t.Fatalf("sendAgent: %v", err)
	}

	if !strings.Contains(output, "checking the forecast") {
		t.Fatalf("real reasoning content was not rendered: %q", output)
	}
	if strings.Contains(strings.ReplaceAll(output, "checking the forecast", ""), ".") {
		t.Fatalf("dot-filler reasoning leaked into the output: %q", output)
	}
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

func TestSendAgentRendersToolFinishedAndDelegationMessage(t *testing.T) {
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

		if !strings.Contains(string(raw), "delegate this") {
			return
		}

		frames = []string{
			`{"type":"tool","session_id":"s","status":"started","name":"agent_spawn"}`,
			`{"type":"tool","session_id":"s","status":"started","name":"worker","message":"spawned by naru, running independently"}`,
			`{"type":"tool","session_id":"s","status":"started","name":"worker/bash_exec"}`,
			`{"type":"tool","session_id":"s","status":"finished","name":"worker/bash_exec"}`,
			`{"type":"tool","session_id":"s","status":"finished","name":"worker"}`,
			`{"type":"tool","session_id":"s","status":"finished","name":"agent_spawn"}`,
			`{"type":"chunk","session_id":"s","chunk":{"choices":[{"delta":{"content":"done"}}]}}`,
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
		err = sendAgent(&sh, "delegate this")
	})
	if err != nil {
		t.Fatalf("sendAgent: %v", err)
	}

	if !strings.Contains(output, "spawned by naru, running independently") {
		t.Fatalf("delegation message was not rendered: %q", output)
	}
	if !strings.Contains(output, "✔ worker/bash_exec") {
		t.Fatalf("nested tool's finished line was not rendered: %q", output)
	}
	if !strings.Contains(output, "✔ worker") {
		t.Fatalf("delegate's finished line was not rendered: %q", output)
	}
	if !strings.Contains(output, "✔ agent_spawn") {
		t.Fatalf("outer agent_spawn finished line was not rendered: %q", output)
	}
}

func TestRemoveToolNameDropsTheMostRecentMatch(t *testing.T) {
	var stack []string

	stack = []string{"agent_spawn", "worker", "worker"}
	stack = removeToolName(stack, "worker")

	if len(stack) != 2 || stack[0] != "agent_spawn" || stack[1] != "worker" {
		t.Fatalf("stack = %v, want the last worker removed", stack)
	}
}

func TestConnectSendsAnAttachFrameForTheSession(t *testing.T) {
	var upgrader websocket.Upgrader
	var mux *http.ServeMux
	var server *httptest.Server
	var sh state
	var raw []byte
	var got frame
	var received chan []byte

	var err error

	received = make(chan []byte, 1)

	mux = http.NewServeMux()
	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{{"id": "a1", "name": "naru"}})
	})
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": "s1", "agent_id": "a1"})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		var conn *websocket.Conn
		var body []byte
		var handlerErr error

		conn, handlerErr = upgrader.Upgrade(w, r, nil)
		if handlerErr != nil {
			return
		}
		defer conn.Close()

		_, body, handlerErr = conn.ReadMessage()
		if handlerErr == nil {
			received <- body
		}
	})

	server = httptest.NewServer(mux)
	defer server.Close()

	sh = state{url: "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"}

	err = connect(&sh)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sh.conn.Close()

	if sh.session != "s1" {
		t.Fatalf("sh.session = %q, want %q", sh.session, "s1")
	}

	select {
	case raw = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the attach frame")
	}

	err = json.Unmarshal(raw, &got)
	if err != nil {
		t.Fatalf("unmarshal attach frame: %v", err)
	}
	if got.Type != "attach" || got.SessionId != "s1" {
		t.Fatalf("attach frame = %+v, want type=attach session_id=s1", got)
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

func TestDrainMirrorRendersAnInjectedMessageAndItsReply(t *testing.T) {
	var upgrader websocket.Upgrader
	var server *httptest.Server
	var client *websocket.Conn
	var sh state
	var output string
	var rendered bool

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		var conn *websocket.Conn
		var frames []string
		var payload string

		var handlerErr error

		conn, handlerErr = upgrader.Upgrade(res, req, nil)
		if handlerErr != nil {
			return
		}
		defer conn.Close()

		frames = []string{
			`{"type":"message","session_id":"s","name":"s1","message":"check the build"}`,
			`{"type":"chunk","session_id":"s","chunk":{"choices":[{"delta":{"content":"the build is green"}}]}}`,
			`{"type":"done","session_id":"s"}`,
		}

		for _, payload = range frames {
			handlerErr = conn.WriteMessage(websocket.TextMessage, []byte(payload))
			if handlerErr != nil {
				return
			}
		}

		<-req.Context().Done()
	}))
	defer server.Close()

	client, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	sh = state{conn: client, session: "s"}

	output = captureStdout(t, func() {
		ensureReader(&sh)

		for !rendered {
			rendered = drainMirror(&sh)
		}
	})

	if !strings.Contains(output, "↘ from session s1") {
		t.Fatalf("injected message was not marked with its origin: %q", output)
	}
	if !strings.Contains(output, "check the build") {
		t.Fatalf("injected message body was not rendered: %q", output)
	}
	if !strings.Contains(output, "the build is green") {
		t.Fatalf("mirrored reply was not rendered: %q", output)
	}

	if strings.Index(output, "check the build") > strings.Index(output, "the build is green") {
		t.Fatalf("the injected message should be rendered before the reply: %q", output)
	}

	if sh.mirror == nil || sh.mirror.active {
		t.Fatalf("mirror round should be closed after the done frame")
	}
}

func TestSendAgentFinishesAnOpenMirrorRoundFirst(t *testing.T) {
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

		frames = []string{
			`{"type":"message","session_id":"s","name":"s1","message":"injected question"}`,
			`{"type":"chunk","session_id":"s","chunk":{"choices":[{"delta":{"content":"mirrored answer"}}]}}`,
			`{"type":"done","session_id":"s"}`,
		}

		for _, payload = range frames {
			handlerErr = conn.WriteMessage(websocket.TextMessage, []byte(payload))
			if handlerErr != nil {
				return
			}
		}

		_, raw, handlerErr = conn.ReadMessage()
		if handlerErr != nil || !strings.Contains(string(raw), "my own question") {
			return
		}

		frames = []string{
			`{"type":"chunk","session_id":"s","chunk":{"choices":[{"delta":{"content":"local answer"}}]}}`,
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
		ensureReader(&sh)

		for !drainMirror(&sh) {
		}

		err = sendAgent(&sh, "my own question")
	})
	if err != nil {
		t.Fatalf("sendAgent: %v", err)
	}

	if !strings.Contains(output, "mirrored answer") || !strings.Contains(output, "local answer") {
		t.Fatalf("both rounds should be rendered: %q", output)
	}

	if strings.Index(output, "mirrored answer") > strings.Index(output, "local answer") {
		t.Fatalf("the mirrored round must be closed before the local round: %q", output)
	}
}
