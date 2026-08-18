// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go"
)

func TestOpenAICachePolicyAddsTopLevelControl(t *testing.T) {
	var params openai.ChatCompletionNewParams
	var raw []byte
	var body map[string]any
	var control map[string]any

	var err error

	params.Model = "anthropic/claude-sonnet-4"
	params.Messages = []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hello")}
	applyOpenAICache(&params, &Provider{Cache: CacheEphemeral1h})

	raw, err = json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(raw, &body)
	if err != nil {
		t.Fatal(err)
	}
	control, _ = body["cache_control"].(map[string]any)
	if control["type"] != "ephemeral" || control["ttl"] != "1h" {
		t.Fatalf("cache_control = %#v", control)
	}
}

func TestOpenRouterClaudeAutoEnablesPromptCache(t *testing.T) {
	var params openai.ChatCompletionNewParams
	var raw []byte

	var err error

	params.Model = "anthropic/claude-sonnet-4"
	params.Messages = []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hello")}
	applyOpenAICache(&params, &Provider{BaseURL: "https://openrouter.ai/api/v1", Cache: CacheAuto})

	raw, err = json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"cache_control":{"type":"ephemeral"}`) {
		t.Fatalf("request did not enable automatic Claude caching: %s", raw)
	}
}

func TestAnthropicStreamingReportsCacheUsage(t *testing.T) {
	var srv *httptest.Server
	var provider *Provider
	var agent *NaruAgent
	var result *Completion
	var streamed strings.Builder
	var requestBody map[string]any
	var control map[string]any

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		err = json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `event: message_start`)
		fmt.Fprintln(w, `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":20,"cache_creation_input_tokens":60,"cache_read_input_tokens":40,"output_tokens":0}}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: content_block_start`)
		fmt.Fprintln(w, `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: content_block_delta`)
		fmt.Fprintln(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: content_block_stop`)
		fmt.Fprintln(w, `data: {"type":"content_block_stop","index":0}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: message_delta`)
		fmt.Fprintln(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `event: message_stop`)
		fmt.Fprintln(w, `data: {"type":"message_stop"}`)
		fmt.Fprintln(w)
	}))
	defer srv.Close()

	provider = &Provider{Id: "anthropic-test", Name: "anthropic", Kind: ProviderAnthropic, BaseURL: srv.URL, ApiKey: "test", Cache: CacheEphemeral1h}
	Providers = []*Provider{provider}
	DefaultProvider = provider
	t.Cleanup(func() {
		Providers = nil
		DefaultProvider = nil
	})
	agent = AgentNew("claude", "", "", "claude-test", provider)

	result, err = Complete(context.Background(), agent,
		[]openai.ChatCompletionMessageParamUnion{openai.UserMessage("hello")}, nil, "", func(value string) { streamed.WriteString(value) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "hello" || streamed.String() != "hello" {
		t.Fatalf("content = %q, streamed = %q", result.Content, streamed.String())
	}
	if result.Usage.PromptTokens != 120 || result.Usage.CachedTokens != 40 || result.Usage.CacheWriteTokens != 60 || result.Usage.CompletionTokens != 5 {
		t.Fatalf("usage = %+v", result.Usage)
	}
	control, _ = requestBody["cache_control"].(map[string]any)
	if control["ttl"] != "1h" {
		t.Fatalf("request cache_control = %#v", control)
	}
}

func TestResponseCacheRequiresOpenRouter(t *testing.T) {
	var err error

	err = ProviderValidate(Provider{BaseURL: "https://api.openai.com/v1", ResponseCache: true})
	if err == nil || !strings.Contains(err.Error(), "OpenRouter") {
		t.Fatalf("validation error = %v", err)
	}
}
