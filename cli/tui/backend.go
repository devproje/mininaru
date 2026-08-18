// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"

	"github.com/devproje/mininaru/core"
)

type Backend interface {
	Chat(context.Context, *core.Session, *core.NaruAgent, string, func(string), func(string), core.ToolEventFunc, core.ToolApprovalFunc) (*core.Message, error)
	Compact(context.Context, *core.NaruAgent, *core.Session) (bool, error)
	Usage(string) (*core.UsageTotals, error)
	Context(string) (int64, int64, bool, error)
	ToolCalls(string) ([]*core.ToolCall, error)
}

type localBackend struct{}

func (localBackend) Chat(ctx context.Context, session *core.Session, agent *core.NaruAgent, content string,
	onContent, onReasoning func(string), onTool core.ToolEventFunc, approve core.ToolApprovalFunc) (*core.Message, error) {
	return core.ChatWithApproval(ctx, session, agent, content, onContent, onReasoning, onTool, approve)
}

func (localBackend) Compact(ctx context.Context, agent *core.NaruAgent, session *core.Session) (bool, error) {
	return core.CompactNow(ctx, agent, session)
}

func (localBackend) Usage(sessionId string) (*core.UsageTotals, error) {
	return core.SessionUsage(sessionId)
}

func (localBackend) Context(sessionId string) (int64, int64, bool, error) {
	return core.SessionContextTokens(sessionId)
}

func (localBackend) ToolCalls(messageId string) ([]*core.ToolCall, error) {
	return core.ToolCallList(messageId)
}
