// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"context"
	"sync"
)

var sessionAutoApprove sync.Map

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

type approvalRouter struct {
	mu      sync.Mutex
	pending map[string]chan string
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
