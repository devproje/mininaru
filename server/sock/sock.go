// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go"
)

type inboundFrame struct {
	Type      string `json:"type,omitempty"`
	SessionId string `json:"session_id"`
	Content   string `json:"content,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
	Decision  string `json:"decision,omitempty"`
}

type outboundFrame struct {
	Type      string                      `json:"type"`
	SessionId string                      `json:"session_id"`
	Chunk     *openai.ChatCompletionChunk `json:"chunk,omitempty"`
	Reasoning string                      `json:"reasoning,omitempty"`
	Message   string                      `json:"message,omitempty"`
	Name      string                      `json:"name,omitempty"`
	Status    string                      `json:"status,omitempty"`
	Arguments string                      `json:"arguments,omitempty"`
}

type reasoningChunk struct {
	Choices []struct {
		Delta struct {
			Reasoning        string `json:"reasoning"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
}

type safeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

var upgrader websocket.Upgrader = websocket.Upgrader{
	CheckOrigin: func(req *http.Request) bool {
		return true
	},
}

func (c *safeConn) writeFrame(frame outboundFrame) {
	var err error

	c.mu.Lock()
	defer c.mu.Unlock()

	err = c.conn.WriteJSON(frame)
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

func writeErrorFrame(conn *safeConn, sessionId string, message string) {
	conn.writeFrame(outboundFrame{Type: "error", SessionId: sessionId, Message: message})
}

func approveFunc(conn *safeConn, router *approvalRouter, sessionId, anchor string) core.ApproveFunc {
	return func(ctx context.Context, name, arguments string) (string, error) {
		var mode string
		var decision string

		mode = core.YoloLookup(anchor)
		if mode == core.YoloOn || mode == core.YoloPersist {
			return "once", nil
		}
		if sessionApproved(sessionId) {
			return "once", nil
		}

		conn.writeFrame(outboundFrame{Type: "approval_request", SessionId: sessionId, Name: name, Arguments: arguments})

		decision = router.wait(ctx, sessionId)
		if decision == "session" {
			setSessionApproved(sessionId)
		}

		return decision, nil
	}
}

func handleFrame(ctx context.Context, remoteAddr string, conn *safeConn, frame inboundFrame, router *approvalRouter, seen *sync.Map) {
	var session *core.Session
	var agent *core.Agent
	var msg core.Message
	var anchor string
	var unlock func()

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

	registerLiveConn(session.Id, conn)
	seen.Store(session.Id, struct{}{})

	unlock = core.SessionLock(session.Id)
	defer unlock()

	msg = core.Message{Id: uuid.NewString(), SessionId: session.Id, Role: "user", Content: frame.Content}

	err = core.MessageCreate(&msg)
	if err != nil {
		writeErrorFrame(conn, frame.SessionId, err.Error())
		return
	}

	anchor = core.ResolveAnchor(remoteAddr, frame.Cwd)

	err = core.SendChatMessage(ctx, agent, session, anchor, 0, func(chunk openai.ChatCompletionChunk) {
		conn.writeFrame(outboundFrame{Type: "chunk", SessionId: session.Id, Chunk: &chunk, Reasoning: chunkReasoning(chunk)})
	}, func(name, status, message string) {
		conn.writeFrame(outboundFrame{Type: "tool", SessionId: session.Id, Name: name, Status: status, Message: message})
	}, approveFunc(conn, router, session.Id, anchor))
	if err != nil {
		writeErrorFrame(conn, session.Id, err.Error())
		return
	}

	conn.writeFrame(outboundFrame{Type: "done", SessionId: session.Id})
}

func SockHandler(ctx *gin.Context) {
	var wsConn *websocket.Conn
	var conn *safeConn
	var message []byte
	var frame inboundFrame
	var router *approvalRouter
	var remoteAddr string
	var handlerCtx context.Context
	var cancel context.CancelFunc
	var wg sync.WaitGroup
	var seen sync.Map

	var err error

	wsConn, err = upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		util.Log.Error("sock upgrade error", "error", err)
		return
	}

	conn = &safeConn{conn: wsConn}
	router = newApprovalRouter()
	remoteAddr = ctx.Request.RemoteAddr
	handlerCtx, cancel = context.WithCancel(ctx.Request.Context())

	defer wsConn.Close()
	defer func() {
		seen.Range(func(key, value any) bool {
			unregisterLiveConn(key.(string), conn)
			return true
		})
	}()
	defer wg.Wait()
	defer cancel()

	for {
		_, message, err = wsConn.ReadMessage()
		if err != nil {
			util.Log.Info("sock closed", "error", err)
			break
		}

		err = json.Unmarshal(message, &frame)
		if err != nil {
			writeErrorFrame(conn, "", "invalid frame")
			continue
		}

		if frame.Type == "approval" {
			router.deliver(frame.SessionId, frame.Decision)
			continue
		}

		wg.Add(1)
		go func(f inboundFrame) {
			defer wg.Done()
			handleFrame(handlerCtx, remoteAddr, conn, f, router, &seen)
		}(frame)
	}
}
