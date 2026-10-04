// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"context"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/modules/ask_question"
	"github.com/devproje/mininaru/modules/bash"
	"github.com/devproje/mininaru/modules/browser"
	"github.com/devproje/mininaru/modules/file"
	"github.com/devproje/mininaru/modules/mcp"
	"github.com/devproje/mininaru/modules/memory"
	"github.com/devproje/mininaru/modules/skill"
	"github.com/devproje/mininaru/modules/web_fetch"
	"github.com/devproje/mininaru/modules/web_search"
)

func resolveWebBackend() (*modules.WebBackend, error) {
	var prov *WebProvider

	var err error

	prov, err = WebProviderSelected()
	if err != nil {
		return nil, err
	}

	return &modules.WebBackend{Kind: prov.Kind, ApiKey: prov.ApiKey, BaseUrl: prov.BaseUrl}, nil
}

func buildTools(root, sessionId string, caller *Agent, depth int, onTool func(name, status, message string), approve ApproveFunc, ask AskFunc) []modules.Tool {
	var mode string
	var unrestricted bool
	var tools []modules.Tool
	var askQuestion modules.AskFunc

	if root != "" {
		mode = ModeLookup(root)
		unrestricted = mode == ModeFullAuto
		tools = append(tools, bash.Exec(root), file.Read(root, unrestricted), file.Write(root, unrestricted), file.Edit(root, unrestricted))
	}

	if ask != nil {
		askQuestion = func(ctx context.Context, question string, options []string) (string, error) {
			return ask(ctx, sessionId, question, options)
		}
	}

	tools = append(tools, browser.Tools(sessionId)...)
	tools = append(tools, web_search.Tools(resolveWebBackend)...)
	tools = append(tools, web_fetch.Tools(resolveWebBackend)...)
	tools = append(tools, ask_question.Tools(askQuestion)...)
	tools = append(tools, mcp.Tools()...)
	tools = append(tools, memory.Tools(caller.Id)...)
	tools = append(tools, skill.Tool(), skill.CreateTool())
	tools = append(tools, sessionListTool(caller, sessionId), agentListTool())

	if mode == ModePlan && askQuestion != nil {
		tools = append(tools, planApprovalTool(sessionId, onTool, askQuestion))
	}

	if depth < maxSpawnDepth {
		tools = append(tools, agentSpawnTool(caller, root, depth, onTool, approve, ask))
		tools = append(tools, sessionSendTool(caller, sessionId, depth, onTool, approve, ask))
	}

	return tools
}
