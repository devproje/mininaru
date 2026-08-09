// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"os"
	"sync"

	"github.com/devproje/mininaru/util"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type builtinTool struct {
	Build       func() Def
	Permission  Permission
	Annotations mcp.ToolAnnotations
}

var builtinOnce sync.Once

var builtinSession *mcp.ClientSession

var builtinCache []Def

func hint(value bool) *bool {
	return &value
}

func builtinRoot() string {
	var root string

	root = workingRoot
	if root == "" {
		root, _ = os.Getwd()
	}

	return root
}

func builtinTools() []builtinTool {
	return []builtinTool{
		{
			Build:      CurrentTime,
			Permission: PermissionSafe,
			Annotations: mcp.ToolAnnotations{
				Title: "current time", ReadOnlyHint: true, IdempotentHint: false, OpenWorldHint: hint(false),
			},
		},
		{
			Build:      func() Def { return FileRead(builtinRoot()) },
			Permission: PermissionDangerous,
			Annotations: mcp.ToolAnnotations{
				Title: "read file", ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: hint(false),
			},
		},
		{
			Build:      func() Def { return FileWrite(builtinRoot()) },
			Permission: PermissionDangerous,
			Annotations: mcp.ToolAnnotations{
				Title: "write file", ReadOnlyHint: false, DestructiveHint: hint(true), OpenWorldHint: hint(false),
			},
		},
		{
			Build:      func() Def { return BashExec(builtinRoot()) },
			Permission: PermissionDangerous,
			Annotations: mcp.ToolAnnotations{
				Title: "run bash", ReadOnlyHint: false, DestructiveHint: hint(true), OpenWorldHint: hint(true),
			},
		},
		{
			Build:      WebSearch,
			Permission: PermissionSafe,
			Annotations: mcp.ToolAnnotations{
				Title: "web search", ReadOnlyHint: true, IdempotentHint: false, OpenWorldHint: hint(true),
			},
		},
		{
			Build:      WebFetch,
			Permission: PermissionSafe,
			Annotations: mcp.ToolAnnotations{
				Title: "fetch url", ReadOnlyHint: true, IdempotentHint: false, OpenWorldHint: hint(true),
			},
		},
		{
			Build:      SkillLoad,
			Permission: PermissionSafe,
			Annotations: mcp.ToolAnnotations{
				Title: "load skill", ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: hint(false),
			},
		},
		{
			Build:      Memory,
			Permission: PermissionPrivileged,
			Annotations: mcp.ToolAnnotations{
				Title: "manage memory", ReadOnlyHint: false, DestructiveHint: hint(true), OpenWorldHint: hint(false),
			},
		},
	}
}

func builtinHandler(tool builtinTool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var arguments string
		var output string
		var text string

		var err error

		if req.Params != nil {
			arguments = string(req.Params.Arguments)
		}

		output, err = tool.Build().Execute(ctx, arguments)
		if err == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: output}}}, nil
		}

		text = err.Error()
		if output != "" {
			text = text + "\n" + output
		}

		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
	}
}

func newBuiltinServer() *mcp.Server {
	var server *mcp.Server
	var tool builtinTool
	var def Def

	server = mcp.NewServer(&mcp.Implementation{Name: "mininaru-builtin", Version: util.AppVersion}, nil)

	for _, tool = range builtinTools() {
		def = tool.Build()

		server.AddTool(&mcp.Tool{
			Name:        def.Name,
			Description: def.Description,
			InputSchema: def.Parameters,
			Annotations: &tool.Annotations,
		}, builtinHandler(tool))
	}

	return server
}

func builtinConnect() error {
	var serverTransport *mcp.InMemoryTransport
	var clientTransport *mcp.InMemoryTransport
	var client *mcp.Client

	var err error

	serverTransport, clientTransport = mcp.NewInMemoryTransports()

	_, err = newBuiltinServer().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		return err
	}

	client = mcp.NewClient(&mcp.Implementation{Name: "mininaru", Version: util.AppVersion}, nil)

	builtinSession, err = client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		return err
	}

	return nil
}

func builtinDefs() []Def {
	var permissions map[string]Permission
	var tool builtinTool
	var result *mcp.ListToolsResult
	var listed *mcp.Tool

	var err error

	builtinOnce.Do(func() {
		err = builtinConnect()
		if err != nil {
			util.Log.Error("builtin mcp server unavailable", "error", err)
			return
		}

		permissions = make(map[string]Permission)
		for _, tool = range builtinTools() {
			permissions[tool.Build().Name] = tool.Permission
		}

		result, err = builtinSession.ListTools(context.Background(), nil)
		if err != nil {
			util.Log.Error("builtin mcp tool listing failed", "error", err)
			return
		}

		for _, listed = range result.Tools {
			builtinCache = append(builtinCache, Def{
				Name:        listed.Name,
				Description: listed.Description,
				Parameters:  schemaObject(listed.InputSchema),
				Permission:  permissions[listed.Name],
				daemon:      true,
				Execute:     sessionExecute(builtinSession, listed.Name),
			})
		}
	})

	return builtinCache
}
