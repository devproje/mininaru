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
	Agent     string `json:"agent"`
	Current   bool   `json:"current"`
	CreatedAt string `json:"created_at"`
}

type agentSummary struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model"`
}

func sessionListTool(caller *Agent, callerSessionId string) modules.Tool {
	return modules.Tool{
		Name: SessionListToolName,
		Description: "List every session, across every agent, that currently has a live viewer connected — " +
			"marks which one is this conversation. Only sessions owned by the calling agent are valid " +
			"session_send targets; the rest are shown for awareness of what else is active right now.",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var sessions []*Session
			var agents []*Agent
			var agentNames map[string]string
			var agent *Agent
			var live []string
			var liveSet map[string]bool
			var item *Session
			var id string
			var name string
			var ok bool
			var summaries []sessionSummary
			var buf []byte

			var err error

			sessions, err = SessionListAll()
			if err != nil {
				return "", err
			}

			agents, err = AgentList()
			if err != nil {
				return "", err
			}

			agentNames = make(map[string]string, len(agents))
			for _, agent = range agents {
				agentNames[agent.Id] = agent.Name
			}

			if liveSessionIds != nil {
				live = liveSessionIds()
			}

			liveSet = make(map[string]bool, len(live))
			for _, id = range live {
				liveSet[id] = true
			}

			for _, item = range sessions {
				if !liveSet[item.Id] {
					continue
				}

				name, ok = agentNames[item.AgentId]
				if !ok {
					name = item.AgentId
				}

				summaries = append(summaries, sessionSummary{
					Id: item.Id, Name: item.Name, Agent: name,
					Current: item.Id == callerSessionId, CreatedAt: item.CreatedAt,
				})
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
