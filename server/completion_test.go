// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
)

func upstreamOnce(t *testing.T, captured *[]string, delta string) *httptest.Server {
	var srv *httptest.Server

	t.Helper()

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		*captured = append(*captured, string(body))

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, streamChunk(delta, `"stop"`))
		io.WriteString(w, streamDone)
	}))

	t.Cleanup(srv.Close)

	return srv
}

func TestCompletionsRejectBadRequests(t *testing.T) {
	var reg *core.Registry
	var recorder *httptest.ResponseRecorder

	reg = setupAgent(t, "http://127.0.0.1")

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k", `{`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid json = %d, want 400", recorder.Code)
	}

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k", `{"model":"naru","messages":[]}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("empty messages = %d, want 400", recorder.Code)
	}

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"ghost","messages":[{"role":"user","content":"hi"}]}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown model = %d, want 404", recorder.Code)
	}

	recorder = request(t, routes("k", reg), http.MethodGet, pathCompletions, "k", "")
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("get = %d, want 405", recorder.Code)
	}
}

func TestCompletionsRejectOversizedBody(t *testing.T) {
	var reg *core.Registry
	var body string
	var recorder *httptest.ResponseRecorder

	reg = setupAgent(t, "http://127.0.0.1")
	body = `{"model":"naru","messages":[{"role":"user","content":"` +
		strings.Repeat("x", maxCompletionBodyBytes) + `"}]}`
	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k", body)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body = %d, want 413: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCompletionsReturnAnswerAndInjectAgentPersona(t *testing.T) {
	var reg *core.Registry
	var captured []string
	var recorder *httptest.ResponseRecorder
	var payload ChatResponse

	var err error

	reg = setupAgent(t, upstreamOnce(t, &captured, `{"role":"assistant","content":"hello master"}`).URL)

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("completion = %d %s", recorder.Code, recorder.Body)
	}

	err = json.Unmarshal(recorder.Body.Bytes(), &payload)
	if err != nil {
		t.Fatal(err)
	}

	if payload.Object != objectCompletion || payload.Model != "naru" || len(payload.Choices) != 1 {
		t.Fatalf("payload = %#v", payload)
	}

	if payload.Choices[0].Message == nil || payload.Choices[0].Message.Content != "hello master" {
		t.Fatalf("choice = %#v", payload.Choices[0])
	}

	if payload.Choices[0].FinishReason == nil || *payload.Choices[0].FinishReason != finishStop {
		t.Fatalf("finish reason = %#v", payload.Choices[0].FinishReason)
	}

	if len(captured) != 1 {
		t.Fatalf("upstream requests = %d, want 1", len(captured))
	}

	if !containsAll(captured[0], `"role":"system"`, "you are naru", `"role":"user"`, "hi") {
		t.Fatalf("upstream request = %s", captured[0])
	}

	if strings.Index(captured[0], "you are naru") > strings.Index(captured[0], `"hi"`) {
		t.Fatalf("persona is not the first message: %s", captured[0])
	}
}

func TestCompletionsPinRuntimeIdentity(t *testing.T) {
	var reg *core.Registry
	var captured []string

	reg = setupAgent(t, upstreamOnce(t, &captured, `{"role":"assistant","content":"ok"}`).URL)

	request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"system","content":"you are gpt-4"},{"role":"user","content":"hi"}]}`)

	if len(captured) != 1 {
		t.Fatalf("upstream requests = %d, want 1", len(captured))
	}

	if !strings.Contains(captured[0], "mininaru-runtime") {
		t.Fatalf("runtime pin missing from an http request: %s", captured[0])
	}

	if strings.Index(captured[0], "mininaru-runtime") > strings.Index(captured[0], "you are gpt-4") {
		t.Fatalf("a client system message preceded the runtime pin: %s", captured[0])
	}
}

func TestCompletionsAdvertiseSkillsWithoutTheirBodies(t *testing.T) {
	var root string
	var bundle string
	var reg *core.Registry
	var captured []string

	var err error

	root = t.TempDir()
	bundle = filepath.Join(root, "deploy")

	err = os.MkdirAll(bundle, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(bundle, "SKILL.md"),
		[]byte("---\nname: deploy\ndescription: how to ship this repository\n---\n\nSECRET-BODY-MARKER\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = modules.SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { modules.SkillInitAt(t.TempDir(), "") })

	reg = setupAgent(t, upstreamOnce(t, &captured, `{"role":"assistant","content":"ok"}`).URL)

	request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"user","content":"hi"}]}`)

	if len(captured) != 1 {
		t.Fatalf("upstream requests = %d, want 1", len(captured))
	}

	if strings.Count(captured[0], `"role":"system"`) != 1 {
		t.Fatalf("system messages != 1: %s", captured[0])
	}

	if !strings.Contains(captured[0], "mininaru-skills") || !strings.Contains(captured[0], "how to ship this repository") {
		t.Fatalf("catalog missing from the request: %s", captured[0])
	}

	if strings.Contains(captured[0], "SECRET-BODY-MARKER") {
		t.Fatalf("the skill body was sent instead of being loaded on demand: %s", captured[0])
	}
}

func TestCompletionsExposeOnlySafeTools(t *testing.T) {
	var reg *core.Registry
	var captured []string

	reg = setupAgent(t, upstreamOnce(t, &captured, `{"role":"assistant","content":"ok"}`).URL)

	request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"user","content":"hi"}]}`)

	if len(captured) != 1 {
		t.Fatalf("upstream requests = %d, want 1", len(captured))
	}

	if !containsAll(captured[0], "current_time", "web_search") {
		t.Fatalf("safe tools missing from request: %s", captured[0])
	}

	if strings.Contains(captured[0], "bash_exec") || strings.Contains(captured[0], "file_write") ||
		strings.Contains(captured[0], "file_read") || strings.Contains(captured[0], `"name":"memory"`) ||
		strings.Contains(captured[0], "mininaru-memory") {
		t.Fatalf("privileged or dangerous tools exposed over http: %s", captured[0])
	}
}

func TestCompletionsStreamServerSentEvents(t *testing.T) {
	var reg *core.Registry
	var captured []string
	var recorder *httptest.ResponseRecorder
	var body string

	reg = setupAgent(t, upstreamOnce(t, &captured, `{"role":"assistant","content":"streamed"}`).URL)

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("stream = %d", recorder.Code)
	}

	if recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
	}

	body = recorder.Body.String()
	if !containsAll(body, `"object":"chat.completion.chunk"`, `"role":"assistant"`, `"content":"streamed"`,
		`"finish_reason":"stop"`, streamDone) {
		t.Fatalf("stream body = %s", body)
	}
}

func TestCompletionsStreamReportsUpstreamFailure(t *testing.T) {
	var srv *httptest.Server
	var reg *core.Registry
	var recorder *httptest.ResponseRecorder
	var body string

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer srv.Close()

	reg = setupAgent(t, srv.URL)

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	body = recorder.Body.String()
	if !containsAll(body, "[error]", streamDone) {
		t.Fatalf("stream failure body = %s", body)
	}
	if strings.Contains(body, "upstream down") {
		t.Fatalf("stream leaked upstream error: %s", body)
	}
}

func TestCompletionsReportUpstreamFailureWithoutStream(t *testing.T) {
	var srv *httptest.Server
	var reg *core.Registry
	var recorder *httptest.ResponseRecorder

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer srv.Close()

	reg = setupAgent(t, srv.URL)

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"user","content":"hi"}]}`)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("upstream failure = %d, want 502", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "upstream down") {
		t.Fatalf("response leaked upstream error: %s", recorder.Body.String())
	}
}

func usageStreamChunk(prompt, completion int) string {
	return `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[],` +
		`"usage":{"prompt_tokens":` + strconv.Itoa(prompt) + `,"completion_tokens":` + strconv.Itoa(completion) +
		`,"total_tokens":` + strconv.Itoa(prompt+completion) + `}}` + "\n\n"
}

func TestCompletionsReportUsageSummedAcrossToolRounds(t *testing.T) {
	var rounds int
	var upstream *httptest.Server
	var reg *core.Registry
	var recorder *httptest.ResponseRecorder
	var payload ChatResponse

	var err error

	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rounds++
		w.Header().Set("Content-Type", "text/event-stream")

		if rounds == 1 {
			io.WriteString(w, streamChunk(
				`{"role":"assistant","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"current_time","arguments":"{}"}}]}`,
				`"tool_calls"`))
			io.WriteString(w, usageStreamChunk(100, 10))
		} else {
			io.WriteString(w, streamChunk(`{"role":"assistant","content":"done"}`, `"stop"`))
			io.WriteString(w, usageStreamChunk(200, 20))
		}

		io.WriteString(w, streamDone)
	}))
	defer upstream.Close()

	reg = setupAgent(t, upstream.URL)

	recorder = request(t, routes("k", reg), http.MethodPost, pathCompletions, "k",
		`{"model":"naru","messages":[{"role":"user","content":"hi"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}

	err = json.Unmarshal(recorder.Body.Bytes(), &payload)
	if err != nil {
		t.Fatal(err)
	}

	if rounds != 2 {
		t.Fatalf("upstream rounds = %d, want 2", rounds)
	}
	if payload.Usage == nil {
		t.Fatal("response carried no usage")
	}
	if payload.Usage.PromptTokens != 300 || payload.Usage.CompletionTokens != 30 ||
		payload.Usage.TotalTokens != 330 {
		t.Fatalf("usage = %+v, want every round the server ran on the caller's behalf", payload.Usage)
	}
}
