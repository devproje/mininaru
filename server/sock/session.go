// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"context"
	"sync"

	"github.com/devproje/mininaru/core"
	"github.com/openai/openai-go"
)

type approvalRouter struct {
	mu      sync.Mutex
	pending map[string]chan string
}

var sessionAutoApprove sync.Map

var liveConns sync.Map

func registerLiveConn(sessionId string, conn *safeConn) {
	liveConns.Store(sessionId, conn)
}

func unregisterLiveConn(sessionId string, conn *safeConn) {
	var stored any
	var ok bool

	stored, ok = liveConns.Load(sessionId)
	if !ok || stored.(*safeConn) != conn {
		return
	}

	liveConns.Delete(sessionId)
}

func lookupLiveConn(sessionId string) (*safeConn, bool) {
	var stored any
	var ok bool

	stored, ok = liveConns.Load(sessionId)
	if !ok {
		return nil, false
	}

	return stored.(*safeConn), true
}

func liveSessionIds() []string {
	var ids []string

	liveConns.Range(func(key, value any) bool {
		ids = append(ids, key.(string))
		return true
	})

	return ids
}

func init() {
	core.SetLiveSessionsLister(liveSessionIds)

	core.SetSessionRouter(func(sessionId, origin, content string) {
		var conn *safeConn
		var ok bool

		conn, ok = lookupLiveConn(sessionId)
		if !ok {
			return
		}

		conn.writeFrame(outboundFrame{Type: "message", SessionId: sessionId, Name: origin, Message: content})
	}, func(sessionId string, chunk openai.ChatCompletionChunk) {
		var conn *safeConn
		var ok bool

		conn, ok = lookupLiveConn(sessionId)
		if !ok {
			return
		}

		conn.writeFrame(outboundFrame{Type: "chunk", SessionId: sessionId, Chunk: &chunk, Reasoning: chunkReasoning(chunk)})
	}, func(sessionId, name, status, message string) {
		var conn *safeConn
		var ok bool

		conn, ok = lookupLiveConn(sessionId)
		if !ok {
			return
		}

		conn.writeFrame(outboundFrame{Type: "tool", SessionId: sessionId, Name: name, Status: status, Message: message})
	}, func(sessionId, failure string) {
		var conn *safeConn
		var ok bool

		conn, ok = lookupLiveConn(sessionId)
		if !ok {
			return
		}

		if failure != "" {
			conn.writeFrame(outboundFrame{Type: "error", SessionId: sessionId, Message: failure})
			return
		}

		conn.writeFrame(outboundFrame{Type: "done", SessionId: sessionId})
	})
}

func sessionApproved(sessionId string) bool {
	var value any
	var ok bool

	value, ok = sessionAutoApprove.Load(sessionId)
	if !ok {
		return false
	}

	return value.(bool)
}

func setSessionApproved(sessionId string) {
	sessionAutoApprove.Store(sessionId, true)
}

func newApprovalRouter() *approvalRouter {
	return &approvalRouter{pending: make(map[string]chan string)}
}

func (r *approvalRouter) wait(ctx context.Context, sessionId string) string {
	var ch chan string
	var decision string

	r.mu.Lock()
	ch = make(chan string, 1)
	r.pending[sessionId] = ch
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.pending, sessionId)
		r.mu.Unlock()
	}()

	select {
	case decision = <-ch:
		return decision
	case <-ctx.Done():
		return "deny"
	}
}

func (r *approvalRouter) deliver(sessionId, decision string) {
	var ch chan string

	r.mu.Lock()
	ch = r.pending[sessionId]
	r.mu.Unlock()

	if ch == nil {
		return
	}

	select {
	case ch <- decision:
	default:
	}
}
