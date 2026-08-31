// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
)

func setupTestProviderAgent(t *testing.T) (string, *httptest.Server) {
	var upstream *httptest.Server
	var err error

	t.Helper()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		var stream bool
		var flusher http.Flusher
		var content string

		json.NewDecoder(r.Body).Decode(&body)
		stream, _ = body["stream"].(bool)

		if !stream {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"c1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}]}`)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher = w.(http.Flusher)
		for _, content = range []string{"Hel", "lo"} {
			fmt.Fprintf(w, "data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n", content)
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

	return "naru", upstream
}

func TestChatCompletionsNonStream(t *testing.T) {
	var router *gin.Engine
	var agentName string
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var resp map[string]any
	var choices []any
	var message map[string]any

	setupTestDB(t)
	router = newRouter()
	agentName, _ = setupTestProviderAgent(t)

	body, _ = json.Marshal(map[string]any{
		"model":    agentName,
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["model"] != agentName {
		t.Fatalf("model = %v, want %q", resp["model"], agentName)
	}

	choices = resp["choices"].([]any)
	message = choices[0].(map[string]any)["message"].(map[string]any)
	if message["content"] != "hi there" {
		t.Fatalf("content = %v, want %q", message["content"], "hi there")
	}
}

func TestChatCompletionsStream(t *testing.T) {
	var router *gin.Engine
	var agentName string
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var out string

	setupTestDB(t)
	router = newRouter()
	agentName, _ = setupTestProviderAgent(t)

	body, _ = json.Marshal(map[string]any{
		"model":    agentName,
		"stream":   true,
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", w.Header().Get("Content-Type"))
	}

	out = w.Body.String()
	if !strings.Contains(out, `"content":"Hel"`) {
		t.Fatalf("stream body missing first chunk: %s", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "data: [DONE]") {
		t.Fatalf("stream body did not end with [DONE]: %s", out)
	}
}

func TestChatCompletionsUnknownModel(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var resp map[string]any
	var errObj map[string]any
	var ok bool

	setupTestDB(t)
	router = newRouter()
	setupTestProviderAgent(t)

	body, _ = json.Marshal(map[string]any{
		"model":    "missing",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	errObj, ok = resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no nested error object: %s", w.Body.String())
	}
	if errObj["type"] != "invalid_request_error" {
		t.Fatalf("error.type = %v, want invalid_request_error", errObj["type"])
	}
}

func TestModelsListsAgents(t *testing.T) {
	var router *gin.Engine
	var agentName string
	var w *httptest.ResponseRecorder
	var req *http.Request
	var resp map[string]any
	var data []any

	setupTestDB(t)
	router = newRouter()
	agentName, _ = setupTestProviderAgent(t)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	data = resp["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data = %d models, want 1", len(data))
	}
	if data[0].(map[string]any)["id"] != agentName {
		t.Fatalf("model id = %v, want %q", data[0].(map[string]any)["id"], agentName)
	}
}

func TestParseOpenAIContent(t *testing.T) {
	var text string
	var images []string

	var err error

	text, images, err = parseOpenAIContent(json.RawMessage(`"just text"`))
	if err != nil || text != "just text" || images != nil {
		t.Fatalf("string: %q %v %v", text, images, err)
	}

	text, images, err = parseOpenAIContent(json.RawMessage(`[{"type":"text","text":"look "},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}}]`))
	if err != nil {
		t.Fatal(err)
	}
	if text != "look " || len(images) != 1 || images[0] != "data:image/png;base64,AAA" {
		t.Fatalf("parts: %q %v", text, images)
	}

	_, _, err = parseOpenAIContent(json.RawMessage(`42`))
	if err == nil {
		t.Fatal("a bare number should be rejected")
	}
}

func TestChatCompletionsAcceptsImageParts(t *testing.T) {
	var router *gin.Engine
	var upstream *httptest.Server
	var w *httptest.ResponseRecorder
	var req *http.Request
	var seen map[string]any
	var body []byte
	var messages []any
	var parts []any

	var err error

	setupTestDB(t)
	router = newRouter()

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&seen)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"c1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"a red square"},"finish_reason":"stop"}]}`)
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

	body, _ = json.Marshal(map[string]any{
		"model": "naru",
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "what colour"},
				{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64,AAAA"}},
			},
		}},
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	messages = seen["messages"].([]any)
	parts = messages[0].(map[string]any)["content"].([]any)
	if len(parts) != 2 || parts[1].(map[string]any)["type"] != "image_url" {
		t.Fatalf("upstream did not receive image parts: %+v", parts)
	}
}
