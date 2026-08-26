// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"

	"github.com/devproje/mininaru/modules"
)

const SessionListToolName = "session_list"

const AgentListToolName = "agent_list"

type sessionSummary struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type agentSummary struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model"`
}

func sessionListTool(caller *Agent) modules.Tool {
	return modules.Tool{
		Name:        SessionListToolName,
		Description: "List the calling agent's own sessions that currently have a live viewer connected — the valid targets for session_send.",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var sessions []*Session
			var live []string
			var liveSet map[string]bool
			var item *Session
			var id string
			var summaries []sessionSummary
			var buf []byte

			var err error

			sessions, err = SessionList(caller.Id)
			if err != nil {
				return "", err
			}

			if liveSessionIds != nil {
				live = liveSessionIds()
			}

			liveSet = make(map[string]bool, len(live))
			for _, id = range live {
				liveSet[id] = true
			}

			for _, item = range sessions {
				if liveSet[item.Id] {
					summaries = append(summaries, sessionSummary{Id: item.Id, Name: item.Name, CreatedAt: item.CreatedAt})
				}
			}

			buf, err = json.Marshal(summaries)
			if err != nil {
				return "", err
			}

			return string(buf), nil
		},
	}
}

func agentListTool() modules.Tool {
	return modules.Tool{
		Name:        AgentListToolName,
		Description: "List every configured agent — the valid targets for agent_spawn.",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var agents []*Agent
			var item *Agent
			var summaries []agentSummary
			var buf []byte

			var err error

			agents, err = AgentList()
			if err != nil {
				return "", err
			}

			for _, item = range agents {
				summaries = append(summaries, agentSummary{Id: item.Id, Name: item.Name, Model: item.Model})
			}

			buf, err = json.Marshal(summaries)
			if err != nil {
				return "", err
			}

			return string(buf), nil
		},
	}
}
