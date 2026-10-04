// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
)

const PlanApprovalToolName = "plan_approval"

func planApprovalTool(sessionId string, onTool func(name, status, message string), ask modules.AskFunc) modules.Tool {
	return modules.Tool{
		Name: PlanApprovalToolName,
		Description: "Plan mode is active: read-only tools (file_read, browser_read, browser_screenshot) still " +
			"ask the operator for approval, but mutating ones (bash_exec, file_write, file_edit, the rest of " +
			"browser_*, agent_spawn, session_send) are rejected automatically. Use this once you have a concrete " +
			"plan to present it to the operator and ask how to proceed for the rest of this turn: Allow once " +
			"(ask before each dangerous call from here on), Allow session / Persist (file_read/file_write/" +
			"file_edit auto-run inside this directory), or Deny (stay in Plan and stop here).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"plan": map[string]any{"type": "string", "description": "What you intend to do, for the operator to review."},
			},
			"required":             []string{"plan"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Plan string `json:"plan"`
			}
			var answer string

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Plan == "" {
				return "", fmt.Errorf("plan is required")
			}
			if ask == nil {
				return "", fmt.Errorf("plan_approval has no interactive operator to ask")
			}

			answer, err = ask(ctx, payload.Plan, []string{"Allow once", "Allow session (Persist)", "Deny"})
			if err != nil {
				return "", err
			}

			answer = strings.ToLower(strings.TrimSpace(answer))

			switch {
			case strings.Contains(answer, "once"):
				SetSessionModeOverride(sessionId, ModeDefault)
				if onTool != nil {
					onTool(PlanApprovalToolName, "mode", ModeDefault)
				}

				return "operator chose Allow once — dangerous tools will ask for approval for the rest of this turn", nil
			case strings.Contains(answer, "persist") || strings.Contains(answer, "session"):
				SetSessionModeOverride(sessionId, ModeAutoPersist)
				if onTool != nil {
					onTool(PlanApprovalToolName, "mode", ModeAutoPersist)
				}

				return "operator chose Allow session (Persist) — file_read/file_write/file_edit will auto-run inside this directory for the rest of this turn", nil
			default:
				if cancelSession != nil {
					cancelSession(sessionId)
				}

				return "", fmt.Errorf("plan denied by operator")
			}
		},
	}
}
