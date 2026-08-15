// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type subagentKey struct{}

type subagentPolicy struct {
	CallerId        string
	SessionId       string
	Defs            []modules.Def
	AllowDangerous  bool
	AllowPrivileged bool
	Approve         ToolApprovalFunc
	Depth           int
}

const AgentToolName = "agent_call"

const maxSubagentDepth = 1

var agentToolInstalled bool

func subagentContext(ctx context.Context, policy subagentPolicy) context.Context {
	return context.WithValue(ctx, subagentKey{}, policy)
}

func subagentPolicyFrom(ctx context.Context) (subagentPolicy, bool) {
	var policy subagentPolicy
	var ok bool

	policy, ok = ctx.Value(subagentKey{}).(subagentPolicy)

	return policy, ok
}

func childDefs(defs []modules.Def) []modules.Def {
	var def modules.Def
	var inherited []modules.Def

	for _, def = range defs {
		if def.Name == AgentToolName {
			continue
		}

		inherited = append(inherited, def)
	}

	return inherited
}

func runSubagent(ctx context.Context, policy subagentPolicy, target *NaruAgent, prompt string) (string, error) {
	var defs []modules.Def
	var params openai.ChatCompletionNewParams
	var run completionRun
	var result *Completion

	var err error

	defs = childDefs(policy.Defs)

	params.Model = target.Model
	params.StreamOptions.IncludeUsage = param.NewOpt(true)
	params.Messages = append(params.Messages, openai.SystemMessage(systemPrompt(target, defs)))
	params.Messages = append(params.Messages, openai.UserMessage(prompt))

	if len(defs) > 0 {
		params.Tools = toolParams(defs)
	}

	run = completionRun{
		AI: target.AI, Params: params, Defs: defs,
		AllowDangerous: policy.AllowDangerous, AllowPrivileged: policy.AllowPrivileged,
		AgentId: target.Id, SessionId: policy.SessionId, Depth: policy.Depth + 1,
		Approve: policy.Approve,
	}

	result, err = run.execute(ctx)
	if err != nil {
		return "", err
	}

	usageRecord(policy.SessionId, "", UsageSubagent, result.Usage)

	return strings.TrimSpace(result.Content), nil
}

func AgentCallTool() modules.Def {
	return modules.Def{
		Name: AgentToolName,
		Description: "Delegate one self-contained task to another configured agent and return its answer. " +
			"The agent starts with no memory of this conversation, so the prompt has to carry everything it needs.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent":  map[string]any{"type": "string"},
				"prompt": map[string]any{"type": "string"},
			},
			"required":             []string{"agent", "prompt"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionPrivileged,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Agent  string `json:"agent"`
				Prompt string `json:"prompt"`
			}
			var policy subagentPolicy
			var ok bool
			var target *NaruAgent
			var answer string

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}

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

			policy, ok = subagentPolicyFrom(ctx)
			if !ok {
				return "", fmt.Errorf("%s is only available inside a chat turn", AgentToolName)
			}
			if policy.Depth >= maxSubagentDepth {
				return "", fmt.Errorf("delegation is limited to %d level deep, and this call is already inside one", maxSubagentDepth)
			}

			target, err = AgentByName(payload.Agent)
			if err != nil {
				return "", err
			}
			if target.Id == policy.CallerId {
				return "", fmt.Errorf("agent %s cannot delegate to itself", target.Name)
			}
			if target.AI == nil {
				return "", fmt.Errorf("agent %s has no available provider client", target.Name)
			}

			util.Log.Debug("delegating to an agent",
				"agent", target.Name, "model", target.Model, "depth", policy.Depth)

			answer, err = runSubagent(ctx, policy, target, payload.Prompt)
			if err != nil {
				return "", fmt.Errorf("agent %s failed: %w", target.Name, err)
			}
			if answer == "" {
				return "", fmt.Errorf("agent %s returned nothing", target.Name)
			}

			return answer, nil
		},
	}
}

func InstallAgentTool() {
	if agentToolInstalled {
		return
	}

	agentToolInstalled = true

	modules.RegisterBuiltin(AgentCallTool, modules.BuiltinHints{
		Title:       "delegate to an agent",
		ReadOnly:    false,
		Destructive: false,
		OpenWorld:   true,
	})
}
