// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

const SessionSendToolName = "session_send"

func resolveSessionRef(ref string) (*Session, error) {
	var target *Session
	var list []*Session
	var item *Session

	var err error

	target, err = SessionRead(ref)
	if err == nil {
		return target, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	list, err = SessionListAll()
	if err != nil {
		return nil, err
	}

	for _, item = range list {
		if item.Name == ref {
			return item, nil
		}
	}

	return nil, fmt.Errorf("no session %q — check session_list for a valid id or name", ref)
}

func markSenderAgent(caller *Agent, targetAgentId, content string) string {
	if targetAgentId == caller.Id {
		return content
	}

	return fmt.Sprintf("[message from agent %q via session_send]\n%s", caller.Name, content)
}

func sessionSendTool(caller *Agent, callerSessionId, anchor string, depth int, onTool func(name, status, message string), approve ApproveFunc) modules.Tool {
	return modules.Tool{
		Name: SessionSendToolName,
		Description: "Inject a message into another already-running session, even one owned by a different " +
			"agent, and return its reply. If a person is currently connected to that session, they see the " +
			"round stream in live, the same as any other turn.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session": map[string]any{"type": "string", "description": "A session id or name from session_list."},
				"content": map[string]any{"type": "string"},
			},
			"required":             []string{"session", "content"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Session string `json:"session"`
				Content string `json:"content"`
			}
			var target *Session
			var targetAgent *Agent
			var content string
			var msg Message
			var unlock func()
			var childOnTool func(name, status, message string)
			var answer string

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			payload.Session = strings.TrimSpace(payload.Session)
			payload.Content = strings.TrimSpace(payload.Content)

			if payload.Session == "" {
				return "", fmt.Errorf("session is required")
			}
			if payload.Content == "" {
				return "", fmt.Errorf("content is required")
			}
			if payload.Session == callerSessionId {
				return "", fmt.Errorf("session_send cannot target its own session")
			}

			target, err = resolveSessionRef(payload.Session)
			if err != nil {
				return "", err
			}

			targetAgent, err = AgentRead(target.AgentId)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return "", fmt.Errorf("agent for session %s no longer exists", target.Id)
				}

				return "", err
			}

			unlock = SessionLock(target.Id)
			defer unlock()

			content = markSenderAgent(caller, target.AgentId, payload.Content)
			msg = Message{Id: uuid.NewString(), SessionId: target.Id, Role: "user", Content: content}
			err = MessageCreate(&msg)
			if err != nil {
				return "", err
			}

			if mirrorMessage != nil {
				mirrorMessage(target.Id, callerSessionId, content)
			}

			childOnTool = func(name, status, message string) {
				if onTool != nil {
					onTool(target.Id+"/"+name, status, message)
				}
				if mirrorTool != nil {
					mirrorTool(target.Id, name, status, message)
				}
			}

			err = SendChatMessage(ctx, targetAgent, target, anchor, depth+1, func(chunk openai.ChatCompletionChunk) {
				if mirrorChunk != nil {
					mirrorChunk(target.Id, chunk)
				}
			}, childOnTool, approve)
			if err != nil {
				if mirrorDone != nil {
					mirrorDone(target.Id, err.Error())
				}

				return "", fmt.Errorf("session %s failed: %w", target.Id, err)
			}

			if mirrorDone != nil {
				mirrorDone(target.Id, "")
			}

			answer, err = lastAssistantMessage(target.Id)
			if err != nil {
				return "", err
			}

			return "delivered — " + targetAgent.Name + " replied: " + answer, nil
		},
	}
}
