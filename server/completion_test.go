package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
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
		strings.Contains(captured[0], "file_read") {
		t.Fatalf("dangerous tools exposed over http: %s", captured[0])
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
	var reg *core.Registry
	var srv *httptest.Server
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
}

func TestCompletionsReportUpstreamFailureWithoutStream(t *testing.T) {
	var reg *core.Registry
	var srv *httptest.Server
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
}
