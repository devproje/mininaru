// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

type inboundFrame struct {
	SessionId string `json:"session_id"`
	Content   string `json:"content"`
}

type outboundFrame struct {
	Type      string                      `json:"type"`
	SessionId string                      `json:"session_id"`
	Chunk     *openai.ChatCompletionChunk `json:"chunk,omitempty"`
	Reasoning string                      `json:"reasoning,omitempty"`
	Message   string                      `json:"message,omitempty"`
}

type reasoningChunk struct {
	Choices []struct {
		Delta struct {
			Reasoning        string `json:"reasoning"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
}

var upgrader websocket.Upgrader = websocket.Upgrader{
	CheckOrigin: func(req *http.Request) bool {
		return true
	},
}

func writeFrame(conn *websocket.Conn, frame outboundFrame) {
	var err error

	err = conn.WriteJSON(frame)
	if err != nil {
		util.Log.Error("sock write error", "error", err)
	}
}

func chunkReasoning(chunk openai.ChatCompletionChunk) string {
	var parsed reasoningChunk

	var err error

	err = json.Unmarshal([]byte(chunk.RawJSON()), &parsed)
	if err != nil || len(parsed.Choices) == 0 {
		return ""
	}

	if parsed.Choices[0].Delta.ReasoningContent != "" {
		return parsed.Choices[0].Delta.ReasoningContent
	}

	return parsed.Choices[0].Delta.Reasoning
}

func writeErrorFrame(conn *websocket.Conn, sessionId string, message string) {
	writeFrame(conn, outboundFrame{Type: "error", SessionId: sessionId, Message: message})
}

func handleFrame(ctx context.Context, conn *websocket.Conn, frame inboundFrame) {
	var session *core.Session
	var agent *core.Agent
	var msg core.Message

	var err error

	if frame.SessionId == "" || frame.Content == "" {
		writeErrorFrame(conn, frame.SessionId, "session_id and content are required")
		return
	}

	session, err = core.SessionRead(frame.SessionId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErrorFrame(conn, frame.SessionId, "session not found")
			return
		}

		writeErrorFrame(conn, frame.SessionId, err.Error())
		return
	}

	agent, err = core.AgentRead(session.AgentId)
	if err != nil {
		writeErrorFrame(conn, frame.SessionId, err.Error())
		return
	}

	msg = core.Message{Id: uuid.NewString(), SessionId: session.Id, Role: "user", Content: frame.Content}

	err = core.MessageCreate(&msg)
	if err != nil {
		writeErrorFrame(conn, frame.SessionId, err.Error())
		return
	}

	err = core.SendChatMessage(ctx, agent, session, func(chunk openai.ChatCompletionChunk) {
		writeFrame(conn, outboundFrame{Type: "chunk", SessionId: session.Id, Chunk: &chunk, Reasoning: chunkReasoning(chunk)})
	})
	if err != nil {
		writeErrorFrame(conn, session.Id, err.Error())
		return
	}

	writeFrame(conn, outboundFrame{Type: "done", SessionId: session.Id})
}

func SockHandler(ctx *gin.Context) {
	var conn *websocket.Conn
	var message []byte
	var frame inboundFrame

	var err error

	conn, err = upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		util.Log.Error("sock upgrade error", "error", err)
		return
	}
	defer conn.Close()

	for {
		_, message, err = conn.ReadMessage()
		if err != nil {
			util.Log.Info("sock closed", "error", err)
			break
		}

		err = json.Unmarshal(message, &frame)
		if err != nil {
			writeErrorFrame(conn, "", "invalid frame")
			continue
		}

		handleFrame(ctx.Request.Context(), conn, frame)
	}
}
