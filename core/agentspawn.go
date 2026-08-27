// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

const AgentSpawnToolName = "agent_spawn"

const maxSpawnDepth = 1

const spawnNamePreviewChars = 60

func spawnSessionName(prompt string) string {
	var preview string

	preview = strings.Join(strings.Fields(prompt), " ")
	if len(preview) > spawnNamePreviewChars {
		preview = preview[:spawnNamePreviewChars] + "…"
	}

	return "spawn: " + preview
}

func lastAssistantMessage(sessionId string) (string, error) {
	var history []*Message
	var item *Message

	var err error

	history, err = MessageList(sessionId)
	if err != nil {
		return "", err
	}

	for len(history) > 0 {
		item = history[len(history)-1]
		if item.Role == "assistant" {
			return strings.TrimSpace(item.Content), nil
		}

		history = history[:len(history)-1]
	}

	return "", fmt.Errorf("agent produced no answer")
}

func agentSpawnTool(caller *Agent, anchor string, depth int, onTool func(name, status, message string), approve ApproveFunc) modules.Tool {
	return modules.Tool{
		Name: AgentSpawnToolName,
		Description: "Delegate one self-contained task to another configured agent and return its final answer. " +
			"The target agent starts with no memory of this conversation, so the prompt must carry everything it " +
			"needs. It runs its own tool-calling turn, subject to the same yolo/HIL approval, but cannot itself " +
			"spawn further agents.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent":  map[string]any{"type": "string"},
				"prompt": map[string]any{"type": "string"},
			},
			"required":             []string{"agent", "prompt"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Agent  string `json:"agent"`
				Prompt string `json:"prompt"`
			}
			var target *Agent
			var session Session
			var msg Message
			var childOnTool func(name, status, message string)
			var answer string

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			payload.Agent = strings.TrimSpace(payload.Agent)
			payload.Prompt = strings.TrimSpace(payload.Prompt)

			if payload.Agent == "" {
				return "", fmt.Errorf("agent is required")
			}
			if payload.Prompt == "" {
				return "", fmt.Errorf("prompt is required")
			}

			target, err = AgentByName(payload.Agent)
			if err != nil {
				return "", err
			}
			if target.Id == caller.Id {
				return "", fmt.Errorf("agent %s cannot spawn itself", target.Name)
			}

			session = Session{Id: uuid.NewString(), AgentId: target.Id, Name: spawnSessionName(payload.Prompt)}
			err = SessionCreate(&session)
			if err != nil {
				return "", err
			}

			msg = Message{Id: uuid.NewString(), SessionId: session.Id, Role: "user", Content: payload.Prompt}
			err = MessageCreate(&msg)
			if err != nil {
				return "", err
			}

			if onTool != nil {
				childOnTool = func(name, status, message string) {
					onTool(target.Name+"/"+name, status, message)
				}

				onTool(target.Name, "started", "spawned by "+caller.Name+", running independently — "+payload.Prompt)
			}

			err = SendChatMessage(ctx, target, &session, anchor, depth+1, func(openai.ChatCompletionChunk) {}, childOnTool, approve)
			if err != nil {
				if onTool != nil {
					onTool(target.Name, "failed", err.Error())
				}

				return "", fmt.Errorf("agent %s failed: %w", target.Name, err)
			}

			answer, err = lastAssistantMessage(session.Id)
			if err != nil {
				if onTool != nil {
					onTool(target.Name, "failed", err.Error())
				}

				return "", err
			}

			if onTool != nil {
				onTool(target.Name, "finished", "")
			}

			return answer, nil
		},
	}
}
