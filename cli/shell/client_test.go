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

func TestReplyDecodesReasoningField(t *testing.T) {
	var got reply

	var err error

	err = json.Unmarshal([]byte(`{"type":"chunk","reasoning":"hmm"}`), &got)
	if err != nil || got.Reasoning != "hmm" {
		t.Fatalf("got %+v, err %v", got, err)
	}
}
