// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

func promptChunk(delta, finish string) string {
	return `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":` +
		delta + `,"finish_reason":` + finish + `}]}` + "\n\n"
}

func promptSetup(t *testing.T, upstream string) (*core.Session, *core.NaruAgent) {
	var agent *core.NaruAgent
	var session *core.Session

	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	core.Providers = nil
	core.DefaultProvider = nil
	core.Agents = nil
	core.ProviderCreate(core.Provider{Name: "p", BaseURL: upstream, ApiKey: "k"})

	agent = core.AgentNew("naru", "", "", "m", core.Providers[0])
	core.Global = agent

	session, err = core.SessionCreate(agent, "prompt")
	if err != nil {
		t.Fatal(err)
	}

	return session, agent
}

func TestPromptContentReadsStdinOnDash(t *testing.T) {
	var got string

	var err error

	got, err = promptContent("literal", strings.NewReader("ignored"))
	if err != nil || got != "literal" {
		t.Fatalf("literal prompt = %q err=%v", got, err)
	}

	got, err = promptContent(stdinPrompt, strings.NewReader("  piped question\n"))
	if err != nil || got != "piped question" {
		t.Fatalf("stdin prompt = %q err=%v", got, err)
	}
}

func TestRunPromptPrintsAnswerToStdoutAndLogsToStderr(t *testing.T) {
	var srv *httptest.Server
	var requests int
	var session *core.Session
	var agent *core.NaruAgent
	var out, logs bytes.Buffer
	var messages []*core.Message

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")

		if requests == 1 {
			io.WriteString(w, promptChunk(`{"role":"assistant","content":"checking","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"current_time","arguments":"{}"}}]}`, `"tool_calls"`))
		} else {
			io.WriteString(w, promptChunk(`{"role":"assistant","content":"the answer"}`, `"stop"`))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = promptSetup(t, srv.URL)
	config.Client.Tools.Enabled = true
	config.Client.Thinking.Show = false

	err = runPrompt(context.Background(), &out, &logs, session, agent, "hi")
	if err != nil {
		t.Fatal(err)
	}

	if out.String() != "the answer\n" {
		t.Fatalf("stdout = %q, want only the final answer", out.String())
	}

	if !strings.Contains(logs.String(), "tool current_time started") ||
		!strings.Contains(logs.String(), "tool current_time completed") {
		t.Fatalf("stderr = %q, want tool progress", logs.String())
	}

	messages, err = core.MessageList(session.Id)
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 || messages[1].Content != "the answer" {
		t.Fatalf("persisted messages = %#v", messages)
	}
}

func TestRunPromptReportsFailure(t *testing.T) {
	var srv *httptest.Server
	var session *core.Session
	var agent *core.NaruAgent
	var out, logs bytes.Buffer

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()

	session, agent = promptSetup(t, srv.URL)

	err = runPrompt(context.Background(), &out, &logs, session, agent, "hi")
	if err == nil {
		t.Fatal("runPrompt hid an upstream failure")
	}

	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing on failure", out.String())
	}
}
