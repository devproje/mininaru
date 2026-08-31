// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go"
)

type openaiMessage struct {
	Role    string          `json:"role" binding:"required"`
	Content json.RawMessage `json:"content" binding:"required"`
}

type openaiContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

type openaiChatRequest struct {
	Model    string          `json:"model" binding:"required"`
	Messages []openaiMessage `json:"messages" binding:"required,dive"`
	Stream   bool            `json:"stream"`
}

func respondOpenAIError(ctx *gin.Context, status int, errType string, message string) {
	ctx.JSON(status, gin.H{"error": gin.H{"message": message, "type": errType}})
}

func parseOpenAIContent(raw json.RawMessage) (string, []string, error) {
	var asString string
	var parts []openaiContentPart
	var part openaiContentPart
	var text strings.Builder
	var images []string

	var err error

	err = json.Unmarshal(raw, &asString)
	if err == nil {
		return asString, nil, nil
	}

	err = json.Unmarshal(raw, &parts)
	if err != nil {
		return "", nil, fmt.Errorf("message content must be a string or an array of content parts")
	}

	for _, part = range parts {
		switch part.Type {
		case "text":
			text.WriteString(part.Text)
		case "image_url":
			images = append(images, part.ImageURL.URL)
		}
	}

	return text.String(), images, nil
}

func openaiChatMessages(messages []openaiMessage) ([]core.ChatMessage, error) {
	var out []core.ChatMessage
	var msg openaiMessage
	var text string
	var images []string

	var err error

	for _, msg = range messages {
		text, images, err = parseOpenAIContent(msg.Content)
		if err != nil {
			return nil, err
		}

		out = append(out, core.ChatMessage{Role: msg.Role, Content: text, Images: images})
	}

	return out, nil
}

func writeOpenAIStreamError(ctx *gin.Context, flusher http.Flusher, err error) {
	var data []byte
	var marshalErr error

	data, marshalErr = json.Marshal(gin.H{"error": gin.H{"message": err.Error(), "type": "api_error"}})
	if marshalErr == nil {
		fmt.Fprintf(ctx.Writer, "data: %s\n\n", data)
	}

	fmt.Fprint(ctx.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

func chatCompletionsStream(ctx *gin.Context, agent *core.Agent, messages []core.ChatMessage) {
	var flusher http.Flusher
	var ok bool
	var err error

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Status(http.StatusOK)

	flusher, ok = ctx.Writer.(http.Flusher)
	if !ok {
		respondOpenAIError(ctx, http.StatusInternalServerError, "api_error", "streaming is not supported by this response writer")
		return
	}

	err = core.ChatCompletionStream(ctx.Request.Context(), agent, messages, func(chunk openai.ChatCompletionChunk) error {
		var data []byte
		var writeErr error

		data, writeErr = json.Marshal(chunk)
		if writeErr != nil {
			return writeErr
		}

		_, writeErr = fmt.Fprintf(ctx.Writer, "data: %s\n\n", data)
		if writeErr != nil {
			return writeErr
		}

		flusher.Flush()

		return nil
	})
	if err != nil {
		writeOpenAIStreamError(ctx, flusher, err)
		return
	}

	fmt.Fprint(ctx.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

func ChatCompletions(ctx *gin.Context) {
	var req openaiChatRequest
	var agent *core.Agent
	var messages []core.ChatMessage
	var resp *openai.ChatCompletion

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		respondOpenAIError(ctx, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	agent, err = core.AgentByName(req.Model)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondOpenAIError(ctx, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("model %q not found", req.Model))
			return
		}

		respondOpenAIError(ctx, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	messages, err = openaiChatMessages(req.Messages)
	if err != nil {
		respondOpenAIError(ctx, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	if !req.Stream {
		resp, err = core.ChatCompletion(ctx.Request.Context(), agent, messages)
		if err != nil {
			respondOpenAIError(ctx, http.StatusInternalServerError, "api_error", err.Error())
			return
		}

		ctx.JSON(http.StatusOK, resp)
		return
	}

	chatCompletionsStream(ctx, agent, messages)
}

func Models(ctx *gin.Context) {
	var agents []*core.Agent
	var data []gin.H
	var agent *core.Agent

	var err error

	agents, err = core.AgentList()
	if err != nil {
		respondOpenAIError(ctx, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	for _, agent = range agents {
		data = append(data, gin.H{"id": agent.Name, "object": "model", "created": 0, "owned_by": "mininaru"})
	}

	ctx.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}
