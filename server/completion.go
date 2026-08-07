package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

const finishStop = "stop"

const streamDone = "data: [DONE]\n\n"

func completionId() string {
	return "chatcmpl-" + uuid.NewString()
}

func completeOnce(ctx context.Context, w http.ResponseWriter, target *core.Instance,
	messages []openai.ChatCompletionMessageParamUnion, thinking string) {
	var result *core.Completion
	var finish string
	var payload ChatResponse

	var err error

	result, err = target.Complete(ctx, messages, thinking, nil, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "upstream_error", err.Error())
		return
	}

	finish = finishStop
	payload = ChatResponse{
		Id:      completionId(),
		Object:  objectCompletion,
		Created: time.Now().Unix(),
		Model:   target.Agent.Name,
		Choices: []Choice{{
			Index:        0,
			Message:      &ResponseMessage{Role: roleAssistant, Content: result.Content},
			FinishReason: &finish,
		}},
		Usage: &Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}

	writeJSON(w, http.StatusOK, payload)
}

func chunkResponse(id, model string, created int64, delta Delta, finish *string) ChatResponse {
	return ChatResponse{
		Id:      id,
		Object:  objectChunk,
		Created: created,
		Model:   model,
		Choices: []Choice{{Index: 0, Delta: &delta, FinishReason: finish}},
	}
}

func sendChunk(w http.ResponseWriter, flusher http.Flusher, payload ChatResponse) {
	var buf []byte

	var err error

	buf, err = json.Marshal(payload)
	if err != nil {
		return
	}

	io.WriteString(w, "data: ")
	w.Write(buf)
	io.WriteString(w, "\n\n")

	flusher.Flush()
}

func completeStream(ctx context.Context, w http.ResponseWriter, target *core.Instance,
	messages []openai.ChatCompletionMessageParamUnion, thinking string) {
	var flusher http.Flusher
	var supported bool
	var id string
	var created int64
	var finish string

	var err error

	flusher, supported = w.(http.Flusher)
	if !supported {
		writeError(w, http.StatusInternalServerError, "api_error", "streaming_unsupported", "response writer cannot stream")
		return
	}

	id = completionId()
	created = time.Now().Unix()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{Role: roleAssistant}, nil))

	_, err = target.Complete(ctx, messages, thinking,
		func(text string) {
			sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{Content: text}, nil))
		},
		func(text string) {
			sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{Reasoning: text}, nil))
		})
	if err != nil {
		sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{Content: "\n\n[error] " + err.Error()}, nil))
	}

	finish = finishStop
	sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{}, &finish))

	io.WriteString(w, streamDone)
	flusher.Flush()
}

func handleCompletions(w http.ResponseWriter, r *http.Request, reg *core.Registry) {
	var req ChatRequest
	var target *core.Instance
	var messages []openai.ChatCompletionMessageParamUnion
	var thinking string

	var err error

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", "only POST is supported")
		return
	}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid_json", err.Error())
		return
	}

	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "model_required", "model is required")
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "messages_required", "at least one message is required")
		return
	}

	target, err = reg.Get(req.Model)
	if err != nil {
		writeError(w, http.StatusNotFound, "invalid_request_error", "model_not_found", err.Error())
		return
	}

	messages = requestMessages(req.Messages)

	thinking = strings.ToLower(req.ReasoningEffort)
	if thinking == "" {
		thinking = config.Client.Thinking.Level
	}

	if req.Stream {
		completeStream(r.Context(), w, target, messages, thinking)
		return
	}

	completeOnce(r.Context(), w, target, messages, thinking)
}
