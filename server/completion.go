package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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

const maxCompletionBodyBytes = 1 << 20

const publicUpstreamError = "upstream request failed"

func completionId() string {
	return "chatcmpl-" + uuid.NewString()
}

func completeOnce(ctx context.Context, w http.ResponseWriter, target *core.Instance,
	messages []openai.ChatCompletionMessageParamUnion, thinking string) {
	var logger *slog.Logger
	var started time.Time
	var result *core.Completion
	var finish string
	var payload ChatResponse

	var err error

	logger = requestLogger(ctx)
	started = time.Now()

	result, err = target.Complete(ctx, messages, thinking, nil, nil)
	if err != nil {
		logger.Error("completion failed",
			"agent", target.Agent.Name, "model", target.Agent.Model,
			"duration_ms", time.Since(started).Milliseconds(), "error", err)
		writeError(w, http.StatusBadGateway, "api_error", "upstream_error", publicUpstreamError)
		return
	}

	logger.Info("completion finished",
		"agent", target.Agent.Name, "model", target.Agent.Model, "stream", false,
		"duration_ms", time.Since(started).Milliseconds(),
		"prompt_tokens", result.Usage.PromptTokens,
		"completion_tokens", result.Usage.CompletionTokens,
		"total_tokens", result.Usage.TotalTokens)

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
	var logger *slog.Logger
	var started time.Time
	var flusher http.Flusher
	var supported bool
	var id string
	var created int64
	var result *core.Completion
	var finish string

	var err error

	logger = requestLogger(ctx)
	started = time.Now()

	flusher, supported = w.(http.Flusher)
	if !supported {
		logger.Error("response writer cannot stream", "agent", target.Agent.Name)
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

	result, err = target.Complete(ctx, messages, thinking,
		func(text string) {
			sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{Content: text}, nil))
		},
		func(text string) {
			sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{Reasoning: text}, nil))
		})
	if err != nil {
		logger.Error("streaming completion failed",
			"agent", target.Agent.Name, "model", target.Agent.Model,
			"duration_ms", time.Since(started).Milliseconds(),
			"client_gone", ctx.Err() != nil, "error", err)
		sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{Content: "\n\n[error] " + publicUpstreamError}, nil))
	} else {
		logger.Info("completion finished",
			"agent", target.Agent.Name, "model", target.Agent.Model, "stream", true,
			"duration_ms", time.Since(started).Milliseconds(),
			"prompt_tokens", result.Usage.PromptTokens,
			"completion_tokens", result.Usage.CompletionTokens,
			"total_tokens", result.Usage.TotalTokens)
	}

	finish = finishStop
	sendChunk(w, flusher, chunkResponse(id, target.Agent.Name, created, Delta{}, &finish))

	io.WriteString(w, streamDone)
	flusher.Flush()
}

func handleCompletions(w http.ResponseWriter, r *http.Request, reg *core.Registry) {
	var req ChatRequest
	var tooLarge *http.MaxBytesError
	var target *core.Instance
	var messages []openai.ChatCompletionMessageParamUnion
	var thinking string

	var err error

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", "only POST is supported")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCompletionBodyBytes)
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request_too_large", "request body exceeds 1 MiB")
			return
		}
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
		requestLogger(r.Context()).Warn("request named an unknown agent", "agent", req.Model)
		writeError(w, http.StatusNotFound, "invalid_request_error", "model_not_found", err.Error())
		return
	}

	messages = requestMessages(req.Messages)

	thinking = strings.ToLower(req.ReasoningEffort)
	if thinking == "" {
		thinking = config.Client.Thinking.Level
	}

	requestLogger(r.Context()).Debug("completion accepted",
		"agent", target.Agent.Name, "model", target.Agent.Model,
		"messages", len(req.Messages), "stream", req.Stream, "reasoning_effort", thinking)

	if req.Stream {
		completeStream(r.Context(), w, target, messages, thinking)
		return
	}

	completeOnce(r.Context(), w, target, messages, thinking)
}
