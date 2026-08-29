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
	"time"

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

const (
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
	writeWait  = 10 * time.Second
)

var upgrader websocket.Upgrader = websocket.Upgrader{
	CheckOrigin: func(req *http.Request) bool {
		return true
	},
}

func (c *safeConn) writeFrame(frame outboundFrame) {
	var err error

	c.mu.Lock()
	defer c.mu.Unlock()

	err = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err != nil {
		util.Log.Error("sock write deadline error", "error", err)
	}

	err = c.conn.WriteJSON(frame)
	if err != nil {
		util.Log.Error("sock write error", "error", err)
	}
}

func (c *safeConn) ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
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

func interruptSession(running *sync.Map, sessionId string) {
	var stored any
	var cancel context.CancelFunc
	var ok bool

	stored, ok = running.Load(sessionId)
	if !ok {
		return
	}

	cancel, ok = stored.(context.CancelFunc)
	if !ok {
		return
	}

	cancel()
}

func handleAttach(conn *safeConn, sessionId string, seen *sync.Map) {
	var err error

	if sessionId == "" {
		writeErrorFrame(conn, sessionId, "session_id is required")
		return
	}

	_, err = core.SessionRead(sessionId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErrorFrame(conn, sessionId, "session not found")
			return
		}

		writeErrorFrame(conn, sessionId, err.Error())
		return
	}

	registerLiveConn(sessionId, conn)
	seen.Store(sessionId, struct{}{})
}

func handleFrame(ctx context.Context, remoteAddr string, conn *safeConn, frame inboundFrame, router *approvalRouter, seen *sync.Map, running *sync.Map) {
	var session *core.Session
	var agent *core.Agent
	var msg core.Message
	var anchor string
	var unlock func()
	var cancel context.CancelFunc

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

	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	running.Store(session.Id, cancel)
	defer running.Delete(session.Id)

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
	var running sync.Map

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

	err = wsConn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		util.Log.Error("sock read deadline error", "error", err)
	}
	wsConn.SetPongHandler(func(string) error {
		return wsConn.SetReadDeadline(time.Now().Add(pongWait))
	})

	wg.Add(1)
	go func() {
		var tick *time.Ticker

		defer wg.Done()

		tick = time.NewTicker(pingPeriod)
		defer tick.Stop()

		for {
			select {
			case <-handlerCtx.Done():
				return
			case <-tick.C:
				if conn.ping() != nil {
					cancel()
					wsConn.Close()
					return
				}
			}
		}
	}()

	for {
		frame = inboundFrame{}

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

		if frame.Type == "interrupt" {
			interruptSession(&running, frame.SessionId)
			continue
		}

		if frame.Type == "approval" {
			router.deliver(frame.SessionId, frame.Decision)
			continue
		}

		if frame.Type == "attach" {
			handleAttach(conn, frame.SessionId, &seen)
			continue
		}

		wg.Add(1)
		go func(f inboundFrame) {
			defer wg.Done()
			handleFrame(handlerCtx, remoteAddr, conn, f, router, &seen, &running)
		}(frame)
	}
}
