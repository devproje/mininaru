// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go"
)

func setupTestScreenshotRoundtrip(t *testing.T, anchor string, secondBody *string) *httptest.Server {
	var upstream *httptest.Server
	var round int

	var err error

	t.Helper()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var flusher http.Flusher
		var body []byte

		round++

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher = w.(http.Flusher)

		if round == 1 {
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\","+
				"\"function\":{\"name\":\"file_read\",\"arguments\":\"{\\\"path\\\":\\\"shot.txt\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
				"\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()

			return
		}

		body, _ = io.ReadAll(r.Body)
		*secondBody = string(body)

		fmt.Fprint(w, "data: {\"id\":\"c2\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,"+
			"\"delta\":{\"content\":\"saw it\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	t.Cleanup(upstream.Close)

	err = os.WriteFile(filepath.Join(anchor, "shot.txt"), []byte("data:image/png;base64,AAAA"), 0600)
	if err != nil {
		t.Fatal(err)
	}

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

func TestSendChatMessageScreenshotBecomesSyntheticImageMessage(t *testing.T) {
	var agent *Agent
	var session *Session
	var anchor string
	var toolCalls []*ToolCall
	var secondBody string

	var err error

	setupTestDB(t)
	anchor = t.TempDir()
	setupTestScreenshotRoundtrip(t, anchor, &secondBody)

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

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "take a screenshot"})
	if err != nil {
		t.Fatal(err)
	}

	err = SendChatMessage(t.Context(), agent, session, anchor, func(chunk openai.ChatCompletionChunk) {}, func(name, status, message string) {},
		func(ctx context.Context, name, arguments string) (string, error) { return "once", nil })
	if err != nil {
		t.Fatalf("SendChatMessage failed: %v", err)
	}

	toolCalls, err = ToolCallList("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(toolCalls))
	}
	if toolCalls[0].Result != "screenshot captured" {
		t.Fatalf("stored tool call result = %q, want the placeholder, not the raw data URL", toolCalls[0].Result)
	}

	if !strings.Contains(secondBody, "image_url") {
		t.Fatalf("second round request is missing an image_url content part: %s", secondBody)
	}
	if !strings.Contains(secondBody, "AAAA") {
		t.Fatalf("second round request is missing the screenshot data: %s", secondBody)
	}
}
