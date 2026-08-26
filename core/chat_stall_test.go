// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go"
)

func setupTestStalledUpstream(t *testing.T) *httptest.Server {
	var upstream *httptest.Server

	var err error

	t.Helper()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()

		<-r.Context().Done()
	}))
	t.Cleanup(upstream.Close)

	err = ProviderCreate(&Provider{Id: "p1", Name: "test", BaseUrl: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}
	err = ProviderActivate("p1")
	if err != nil {
		t.Fatal(err)
	}

	return upstream
}

func TestSendChatMessageFailsOnAStalledStreamInsteadOfHangingForever(t *testing.T) {
	var agent *Agent
	var session *Session
	var previousIdleTimeout time.Duration
	var start time.Time
	var elapsed time.Duration

	var err error

	setupTestDB(t)
	setupTestStalledUpstream(t)

	previousIdleTimeout = streamIdleTimeout
	streamIdleTimeout = 200 * time.Millisecond
	t.Cleanup(func() { streamIdleTimeout = previousIdleTimeout })

	agent = &Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"}
	err = AgentCreate(agent)
	if err != nil {
		t.Fatal(err)
	}

	session = &Session{Id: "s1", AgentId: "a1"}
	err = SessionCreate(session)
	if err != nil {
		t.Fatal(err)
	}

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	start = time.Now()

	err = SendChatMessage(t.Context(), agent, session, t.TempDir(), 0, func(chunk openai.ChatCompletionChunk) {}, func(name, status, message string) {},
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })

	elapsed = time.Since(start)

	if err == nil {
		t.Fatal("expected an idle-timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "idle") {
		t.Fatalf("error = %v, want an idle-timeout error", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("SendChatMessage took %v to fail, want it bounded by the (lowered) idle timeout", elapsed)
	}

	var msg *Message
	msg, err = MessageRead("m1")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Status != "failed" {
		t.Fatalf("message status = %q, want failed", msg.Status)
	}
}
