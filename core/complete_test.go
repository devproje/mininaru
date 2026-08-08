package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	"github.com/openai/openai-go"
)

func rowCount(t *testing.T, table string) int {
	var count int

	var err error

	t.Helper()

	err = util.DB.QueryRow("SELECT count(*) FROM " + table + ";").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	return count
}

func TestCompleteRunsToolsWithoutTouchingDatabase(t *testing.T) {
	var srv *httptest.Server
	var requests []string
	var agent *NaruAgent
	var def modules.Def
	var executions int
	var result *Completion

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "text/event-stream")

		if len(requests) == 1 {
			io.WriteString(w, toolChunk("round-1", `{"role":"assistant","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"echo","arguments":"{}"}}]}`, `"tool_calls"`))
		} else {
			io.WriteString(w, toolChunk("round-2", `{"role":"assistant","content":"done"}`, `"stop"`))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	_, agent = thinkingSetup(t, srv.URL)
	agent.Role = "you are naru"

	def = modules.Def{
		Name: "echo", Description: "echo", Permission: modules.PermissionSafe,
		Parameters: map[string]any{"type": "object"},
		Execute: func(context.Context, string) (string, error) {
			executions++
			return "echoed", nil
		},
	}

	result, err = Complete(context.Background(), agent,
		[]openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")}, []modules.Def{def}, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Content != "done" || executions != 1 || len(requests) != 2 {
		t.Fatalf("content=%q executions=%d requests=%d", result.Content, executions, len(requests))
	}

	if !strings.Contains(requests[0], "you are naru") || !strings.Contains(requests[1], `"role":"tool"`) {
		t.Fatalf("requests = %#v", requests)
	}

	if rowCount(t, "messages") != 0 || rowCount(t, "tool_calls") != 0 {
		t.Fatalf("stateless completion wrote messages=%d tool_calls=%d",
			rowCount(t, "messages"), rowCount(t, "tool_calls"))
	}
}

func TestCompleteRejectsMissingAgentAndMessages(t *testing.T) {
	var agent *NaruAgent

	var err error

	_, err = Complete(context.Background(), nil, []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")}, nil, "", nil, nil)
	if err == nil {
		t.Fatal("complete accepted a nil agent")
	}

	_, agent = thinkingSetup(t, "http://127.0.0.1")

	_, err = Complete(context.Background(), agent, nil, nil, "", nil, nil)
	if err == nil {
		t.Fatal("complete accepted an empty message list")
	}

	agent.AI = nil

	_, err = Complete(context.Background(), agent, []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")}, nil, "", nil, nil)
	if err == nil {
		t.Fatal("complete accepted an agent without a provider client")
	}
}

func TestCompleteKeepsFinalAnswerOnlyAndFullReasoning(t *testing.T) {
	var srv *httptest.Server
	var requests []string
	var agent *NaruAgent
	var def modules.Def
	var result *Completion

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "text/event-stream")

		if len(requests) == 1 {
			io.WriteString(w, toolChunk("round-1", `{"role":"assistant","content":"let me check","reasoning_content":"first thought "}`, `null`))
			io.WriteString(w, toolChunk("round-1", `{"role":"assistant","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"echo","arguments":"{}"}}]}`, `"tool_calls"`))
		} else {
			io.WriteString(w, toolChunk("round-2", `{"role":"assistant","content":"the answer","reasoning_content":"second thought"}`, `"stop"`))
		}

		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	_, agent = thinkingSetup(t, srv.URL)
	def = modules.Def{
		Name: "echo", Permission: modules.PermissionSafe, Parameters: map[string]any{"type": "object"},
		Execute: func(context.Context, string) (string, error) { return "echoed", nil },
	}

	result, err = Complete(context.Background(), agent,
		[]openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")}, []modules.Def{def}, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Content != "the answer" {
		t.Fatalf("content = %q, want only the final round answer", result.Content)
	}

	if result.Reasoning != "first thought second thought" {
		t.Fatalf("reasoning = %q, want the full chain across rounds", result.Reasoning)
	}
}
