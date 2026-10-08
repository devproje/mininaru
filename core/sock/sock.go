// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package sock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devproje/mininaru/modules/store"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
)

type chatSession struct {
	client   openai.Client
	model    string
	messages []openai.ChatCompletionMessageParamUnion
}

type responseFrame struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Message string `json:"message,omitempty"`
}

const completionTimeout = 2 * time.Minute
const maxMessageBytes = 1 << 20

var upgrader websocket.Upgrader

func initializeClient() (*chatSession, error) {
	var profile store.Profile
	var prov store.Provider
	var modelName string
	var opts []option.RequestOption
	var session *chatSession

	var err error

	profile, err = store.GetProfile()
	if err != nil {
		return nil, err
	}
	if profile.Model == nil {
		return nil, fmt.Errorf("profile model is not configured")
	}

	prov, err = store.GetProviderByModel(*profile.Model)
	if err != nil {
		return nil, err
	}
	_, modelName, _ = strings.Cut(*profile.Model, ":")

	opts = append(opts, option.WithAPIKey(prov.ApiKey))
	if prov.BaseUrl != "" {
		opts = append(opts, option.WithBaseURL(prov.BaseUrl))
	}

	session = &chatSession{
		client: openai.NewClient(opts...),
		model:  modelName,
	}
	if profile.Soul != nil && *profile.Soul != "" {
		session.messages = append(session.messages, openai.SystemMessage(*profile.Soul))
	}

	return session, nil
}

func chat(ctx *gin.Context, conn *websocket.Conn, session *chatSession) {
	var messageType int
	var buf []byte
	var content string
	var messages []openai.ChatCompletionMessageParamUnion
	var params openai.ChatCompletionNewParams
	var stream *ssestream.Stream[openai.ChatCompletionChunk]
	var accumulated openai.ChatCompletionAccumulator
	var chunk openai.ChatCompletionChunk
	var invalidChunk bool
	var reply string
	var requestCtx context.Context
	var cancel context.CancelFunc

	var err error

	for {
		messageType, buf, err = conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				util.Log.Warn("websocket read failed", "error", err)
			}
			return
		}
		if messageType != websocket.TextMessage {
			err = conn.WriteJSON(responseFrame{Type: "error", Message: "text messages only"})
			if err != nil {
				return
			}
			continue
		}

		content = strings.TrimSpace(string(buf))
		if content == "" {
			err = conn.WriteJSON(responseFrame{Type: "error", Message: "empty message"})
			if err != nil {
				return
			}
			continue
		}

		messages = make([]openai.ChatCompletionMessageParamUnion, len(session.messages), len(session.messages)+1)
		copy(messages, session.messages)
		messages = append(messages, openai.UserMessage(content))
		params = openai.ChatCompletionNewParams{Model: session.model, Messages: messages}

		requestCtx, cancel = context.WithTimeout(ctx.Request.Context(), completionTimeout)
		stream = session.client.Chat.Completions.NewStreaming(requestCtx, params)
		accumulated = openai.ChatCompletionAccumulator{}
		invalidChunk = false
		for stream.Next() {
			chunk = stream.Current()
			if !accumulated.AddChunk(chunk) {
				invalidChunk = true
				break
			}
			if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
				continue
			}
			if err = conn.WriteJSON(responseFrame{Type: "chunk", Content: chunk.Choices[0].Delta.Content}); err != nil {
				stream.Close()
				cancel()
				return
			}
		}
		err = stream.Err()
		if invalidChunk && err == nil {
			err = fmt.Errorf("invalid chat completion chunk")
		}
		stream.Close()
		cancel()
		if err != nil {
			util.Log.Error("chat completion failed", "error", err)
			err = conn.WriteJSON(responseFrame{Type: "error", Message: "model request failed"})
			if err != nil {
				return
			}
			continue
		}
		if len(accumulated.Choices) == 0 || accumulated.Choices[0].Message.Content == "" {
			err = conn.WriteJSON(responseFrame{Type: "error", Message: "model returned no message"})
			if err != nil {
				return
			}
			continue
		}

		reply = accumulated.Choices[0].Message.Content
		err = conn.WriteJSON(responseFrame{Type: "done"})
		if err != nil {
			return
		}
		session.messages = append(messages, openai.AssistantMessage(reply))
	}
}

func Handler(ctx *gin.Context) {
	var conn *websocket.Conn
	var session *chatSession

	var err error

	session, err = initializeClient()
	if err != nil {
		util.Log.Error("chat client initialization failed", "error", err)
		ctx.JSON(503, gin.H{"error": "chat is not configured"})
		return
	}

	conn, err = upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		util.Log.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(maxMessageBytes)

	chat(ctx, conn, session)
}
