// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type headerTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

type mcpSession struct {
	entry   MCPServer
	session *mcp.ClientSession
	tools   []*mcp.Tool
	defs    []Def
	err     error
}

type MCPStatus struct {
	Name      string
	Transport string
	Error     string
	Connected bool
	Tools     int
}

type mcpManager struct {
	mu       sync.RWMutex
	sessions map[string]*mcpSession
	order    []string
	sources  map[string]string
}

var manager mcpManager = mcpManager{sessions: make(map[string]*mcpSession), sources: make(map[string]string)}

var reloadMu sync.Mutex

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var clone *http.Request
	var key string
	var value string

	clone = req.Clone(req.Context())

	for key, value = range t.headers {
		clone.Header.Set(key, value)
	}

	return t.base.RoundTrip(clone)
}

func headerRoundTripper(headers map[string]string) http.RoundTripper {
	if len(headers) == 0 {
		return http.DefaultTransport
	}

	return &headerTransport{headers: headers, base: http.DefaultTransport}
}

func newTransport(entry *MCPServer) (mcp.Transport, error) {
	var command *exec.Cmd
	var key string
	var value string

	switch entry.Transport {
	case TransportStdio:
		command = exec.Command(entry.Command, entry.Args...)
		command.Dir = entry.Dir
		command.Stderr = os.Stderr
		command.Env = os.Environ()

		for key, value = range entry.Env {
			command.Env = append(command.Env, key+"="+value)
		}

		return &mcp.CommandTransport{Command: command}, nil
	case TransportHTTP:
		return &mcp.StreamableClientTransport{
			Endpoint:   entry.URL,
			HTTPClient: &http.Client{Transport: headerRoundTripper(entry.Headers)},
		}, nil
	}

	return nil, fmt.Errorf("unknown transport %q", entry.Transport)
}

func sessionCall(ctx context.Context, session *mcp.ClientSession, name, arguments string, meta mcp.Meta) (string, error) {
	var params mcp.CallToolParams
	var result *mcp.CallToolResult

	var err error

	params.Name = name
	params.Meta = meta
	if arguments != "" {
		params.Arguments = json.RawMessage(arguments)
	}

	result, err = session.CallTool(ctx, &params)
	if err != nil {
		return "", err
	}

	return resultText(result)
}

func sessionExecute(session *mcp.ClientSession, name string) func(context.Context, string) (string, error) {
	return func(ctx context.Context, arguments string) (string, error) {
		return sessionCall(ctx, session, name, arguments, nil)
	}
}

func listAllTools(ctx context.Context, session *mcp.ClientSession) ([]*mcp.Tool, error) {
	var result *mcp.ListToolsResult
	var params mcp.ListToolsParams
	var tools []*mcp.Tool

	var err error

	for {
		result, err = session.ListTools(ctx, &params)
		if err != nil {
			return nil, err
		}

		tools = append(tools, result.Tools...)

		if result.NextCursor == "" {
			return tools, nil
		}

		params.Cursor = result.NextCursor
	}
}

func mcpDial(ctx context.Context, entry MCPServer) *mcpSession {
	var current mcpSession
	var transport mcp.Transport
	var timeout time.Duration
	var cancel context.CancelFunc
	var client *mcp.Client

	var err error

	current.entry = entry

	transport, err = newTransport(&entry)
	if err != nil {
		current.err = err
		return &current
	}

	timeout = time.Duration(entry.TimeoutSeconds) * time.Second
	if entry.TimeoutSeconds <= 0 {
		timeout = defaultDialTimeout * time.Second
	}

	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	client = mcp.NewClient(&mcp.Implementation{Name: "mininaru", Version: util.AppVersion}, nil)

	current.session, err = client.Connect(ctx, transport, nil)
	if err != nil {
		current.err = err
		return &current
	}

	current.tools, err = listAllTools(ctx, current.session)
	if err != nil {
		current.session.Close()

		current.session = nil
		current.err = err
	}

	return &current
}

func (m *mcpManager) rebind() {
	var taken map[string]bool
	var builtin Def
	var key string
	var current *mcpSession
	var tool *mcp.Tool
	var name string

	taken = make(map[string]bool)
	m.sources = make(map[string]string)

	for _, builtin = range builtinDefs() {
		taken[builtin.Name] = true
		m.sources[builtin.Name] = builtinServerName
	}

	for _, key = range m.order {
		current = m.sessions[key]
		current.defs = nil

		if current.session == nil {
			continue
		}

		for _, tool = range current.tools {
			name = qualifiedName(current.entry.Name, tool.Name)
			if taken[name] {
				util.Log.Warn("dropping a duplicate mcp tool", "tool", name, "server", current.entry.Name)
				continue
			}

			taken[name] = true
			m.sources[name] = current.entry.Name

			current.defs = append(current.defs, Def{
				Name:        name,
				Description: tool.Description,
				Parameters:  schemaObject(tool.InputSchema),
				Permission:  overridePermission(&current.entry, tool.Name, annotationPermission(tool.Annotations)),
				daemon:      serverDaemon(&current.entry),
				Execute:     sessionExecute(current.session, tool.Name),
			})
		}
	}
}

func (m *mcpManager) attach(current *mcpSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sessions[current.entry.Name] == nil {
		m.order = append(m.order, current.entry.Name)
	}

	m.sessions[current.entry.Name] = current

	m.rebind()
}

func (m *mcpManager) defs() []Def {
	var key string
	var defs []Def

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, key = range m.order {
		defs = append(defs, m.sessions[key].defs...)
	}

	return defs
}

func ToolSource(name string) string {
	var source string
	var ok bool

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	source, ok = manager.sources[name]
	if !ok {
		return builtinServerName
	}

	return source
}

func MCPStatusAll() []MCPStatus {
	var key string
	var current *mcpSession
	var status MCPStatus
	var all []MCPStatus

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	for _, key = range manager.order {
		current = manager.sessions[key]
		status = MCPStatus{
			Name:      current.entry.Name,
			Transport: current.entry.Transport,
			Connected: current.session != nil,
			Tools:     len(current.defs),
		}

		if current.err != nil {
			status.Error = current.err.Error()
		}

		all = append(all, status)
	}

	return all
}

func MCPClose() {
	var current *mcpSession

	manager.mu.Lock()
	defer manager.mu.Unlock()

	for _, current = range manager.sessions {
		if current.session == nil {
			continue
		}

		current.session.Close()
	}

	manager.sessions = make(map[string]*mcpSession)
	manager.sources = make(map[string]string)
	manager.order = nil
}

func mcpFingerprint(entry *MCPServer) string {
	var buf []byte

	buf, _ = json.Marshal(entry)

	return string(buf)
}

func MCPReload(ctx context.Context) error {
	var existing map[string]*mcpSession
	var reused map[string]bool
	var dialed map[string]*mcpSession
	var order []string
	var index int
	var name string
	var current *mcpSession
	var stale []*mcpSession

	var err error

	reloadMu.Lock()
	defer reloadMu.Unlock()

	err = MCPLoad()
	if err != nil {
		return err
	}

	existing = make(map[string]*mcpSession)

	manager.mu.RLock()
	for name, current = range manager.sessions {
		existing[name] = current
	}
	manager.mu.RUnlock()

	reused = make(map[string]bool)
	dialed = make(map[string]*mcpSession)

	for index = range MCP.Servers {
		name = MCP.Servers[index].Name
		if !serverEnabled(&MCP.Servers[index]) {
			continue
		}

		order = append(order, name)

		current = existing[name]
		if current != nil && current.session != nil &&
			mcpFingerprint(&current.entry) == mcpFingerprint(&MCP.Servers[index]) {
			reused[name] = true
			continue
		}

		current = mcpDial(ctx, MCP.Servers[index])
		if current.err != nil {
			util.Log.Error("mcp server unavailable", "server", name, "error", current.err)
		}

		dialed[name] = current
	}

	manager.mu.Lock()
	manager.sessions = make(map[string]*mcpSession)

	for _, name = range order {
		if reused[name] {
			manager.sessions[name] = existing[name]
			continue
		}

		manager.sessions[name] = dialed[name]
	}

	manager.order = order
	manager.rebind()
	manager.mu.Unlock()

	for name, current = range existing {
		if reused[name] || current.session == nil {
			continue
		}

		stale = append(stale, current)
	}

	for _, current = range stale {
		current.session.Close()
	}

	return nil
}

func MCPInit(ctx context.Context) error {
	return MCPReload(ctx)
}
