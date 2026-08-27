// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/modules/bash"
	"github.com/devproje/mininaru/modules/browser"
	"github.com/devproje/mininaru/modules/file"
	"github.com/devproje/mininaru/modules/mcp"
	"github.com/devproje/mininaru/modules/memory"
	"github.com/devproje/mininaru/modules/skill"
)

func buildTools(root, sessionId string, caller *Agent, depth int, onTool func(name, status, message string), approve ApproveFunc) []modules.Tool {
	var tools []modules.Tool

	tools = append(tools, bash.Exec(root), file.Read(root), file.Write(root), file.Edit(root))
	tools = append(tools, browser.Tools(sessionId)...)
	tools = append(tools, mcp.Tools()...)
	tools = append(tools, memory.Tools(caller.Id)...)
	tools = append(tools, skill.Tool(), skill.CreateTool())
	tools = append(tools, sessionListTool(caller, sessionId), agentListTool())

	if depth < maxSpawnDepth {
		tools = append(tools, agentSpawnTool(caller, root, depth, onTool, approve))
		tools = append(tools, sessionSendTool(caller, sessionId, root, depth, onTool, approve))
	}

	return tools
}
