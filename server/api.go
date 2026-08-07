package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/openai/openai-go"
)

type RequestMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type ChatRequest struct {
	Model           string           `json:"model"`
	Messages        []RequestMessage `json:"messages"`
	Stream          bool             `json:"stream"`
	ReasoningEffort string           `json:"reasoning_effort"`
}

type ResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Delta struct {
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning_content,omitempty"`
}

type Choice struct {
	Index        int              `json:"index"`
	Message      *ResponseMessage `json:"message,omitempty"`
	Delta        *Delta           `json:"delta,omitempty"`
	FinishReason *string          `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type ChatResponse struct {
	Id      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Model struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type ErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

const (
	objectCompletion = "chat.completion"
	objectChunk      = "chat.completion.chunk"
	objectModel      = "model"
	objectList       = "list"
)

const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
)

func contentText(raw json.RawMessage) string {
	var text string
	var parts []contentPart
	var part contentPart
	var builder strings.Builder

	var err error

	if len(raw) == 0 {
		return ""
	}

	err = json.Unmarshal(raw, &text)
	if err == nil {
		return text
	}

	err = json.Unmarshal(raw, &parts)
	if err != nil {
		return ""
	}

	for _, part = range parts {
		if part.Text == "" {
			continue
		}

		builder.WriteString(part.Text)
	}

	return builder.String()
}

func requestMessages(messages []RequestMessage) []openai.ChatCompletionMessageParamUnion {
	var converted []openai.ChatCompletionMessageParamUnion
	var message RequestMessage
	var text string

	for _, message = range messages {
		text = contentText(message.Content)

		switch message.Role {
		case roleSystem:
			converted = append(converted, openai.SystemMessage(text))
		case roleAssistant:
			converted = append(converted, openai.AssistantMessage(text))
		default:
			converted = append(converted, openai.UserMessage(text))
		}
	}

	return converted
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, kind, code, message string) {
	writeJSON(w, status, ErrorResponse{Error: ErrorBody{Message: message, Type: kind, Code: code}})
}
