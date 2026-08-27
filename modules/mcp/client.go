// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type headerTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

type session struct {
	entry  Server
	client *mcpsdk.ClientSession
	tools  []*mcpsdk.Tool
	defs   []modules.Tool
	err    error
}

type manager struct {
	mu       sync.RWMutex
	sessions map[string]*session
	order    []string
}

type Status struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Tools     int    `json:"tools"`
	Error     string `json:"error"`
}

var shared = manager{sessions: make(map[string]*session)}

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

func newTransport(entry *Server) (mcpsdk.Transport, error) {
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

		return &mcpsdk.CommandTransport{Command: command}, nil
	case TransportHTTP:
		return &mcpsdk.StreamableClientTransport{
			Endpoint:   entry.URL,
			HTTPClient: &http.Client{Transport: headerRoundTripper(entry.Headers)},
		}, nil
	}

	return nil, fmt.Errorf("unknown transport %q", entry.Transport)
}

func call(ctx context.Context, client *mcpsdk.ClientSession, name, arguments string) (string, error) {
	var params mcpsdk.CallToolParams
	var result *mcpsdk.CallToolResult

	var err error

	params.Name = name
	if arguments != "" {
		params.Arguments = json.RawMessage(arguments)
	}

	result, err = client.CallTool(ctx, &params)
	if err != nil {
		return "", err
	}

	return resultText(result)
}

func execute(client *mcpsdk.ClientSession, name string) func(context.Context, string) (string, error) {
	return func(ctx context.Context, arguments string) (string, error) {
		return call(ctx, client, name, arguments)
	}
}

func listAllTools(ctx context.Context, client *mcpsdk.ClientSession) ([]*mcpsdk.Tool, error) {
	var result *mcpsdk.ListToolsResult
	var params mcpsdk.ListToolsParams
	var tools []*mcpsdk.Tool

	var err error

	for {
		result, err = client.ListTools(ctx, &params)
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

func dial(ctx context.Context, entry Server) *session {
	var current session
	var transport mcpsdk.Transport
	var timeout time.Duration
	var cancel context.CancelFunc
	var mcpClient *mcpsdk.Client

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

	mcpClient = mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mininaru", Version: util.AppVersion}, nil)

	current.client, err = mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		current.err = err
		return &current
	}

	current.tools, err = listAllTools(ctx, current.client)
	if err != nil {
		current.client.Close()

		current.client = nil
		current.err = err
	}

	return &current
}

func (m *manager) rebind() {
	var taken map[string]bool
	var key string
	var current *session
	var tool *mcpsdk.Tool
	var name string

	taken = make(map[string]bool)

	for _, key = range m.order {
		current = m.sessions[key]
		current.defs = nil

		if current.client == nil {
			continue
		}

		for _, tool = range current.tools {
			name = qualifiedName(current.entry.Name, tool.Name)
			if taken[name] {
				util.Log.Warn("dropping a duplicate mcp tool", "tool", name, "server", current.entry.Name)
				continue
			}

			taken[name] = true

			current.defs = append(current.defs, modules.Tool{
				Name:        name,
				Description: tool.Description,
				Parameters:  schemaObject(tool.InputSchema),
				Permission:  overridePermission(&current.entry, tool.Name, annotationPermission(tool.Annotations)),
				Execute:     execute(current.client, tool.Name),
			})
		}
	}
}

func Tools() []modules.Tool {
	var key string
	var tools []modules.Tool

	shared.mu.RLock()
	defer shared.mu.RUnlock()

	for _, key = range shared.order {
		tools = append(tools, shared.sessions[key].defs...)
	}

	return tools
}

func statusOf(entry *Server, current *session) Status {
	var errText string

	if current == nil {
		return Status{Name: entry.Name, Transport: entry.Transport, Enabled: false}
	}

	if current.err != nil {
		errText = current.err.Error()
	}

	return Status{
		Name: entry.Name, Transport: entry.Transport, Enabled: true,
		Connected: current.err == nil, Tools: len(current.tools), Error: errText,
	}
}

func StatusAll() []Status {
	var result []Status
	var index int
	var current *session

	shared.mu.RLock()
	defer shared.mu.RUnlock()

	for index = range Loaded.Servers {
		current = shared.sessions[Loaded.Servers[index].Name]
		result = append(result, statusOf(&Loaded.Servers[index], current))
	}

	return result
}

func Close() {
	var current *session

	shared.mu.Lock()
	defer shared.mu.Unlock()

	for _, current = range shared.sessions {
		if current.client == nil {
			continue
		}

		current.client.Close()
	}

	shared.sessions = make(map[string]*session)
	shared.order = nil
}

func fingerprint(entry *Server) string {
	var buf []byte

	buf, _ = json.Marshal(entry)

	return string(buf)
}

func Reload(ctx context.Context) error {
	var existing map[string]*session
	var reused map[string]bool
	var dialed map[string]*session
	var order []string
	var index int
	var name string
	var current *session
	var stale []*session

	var err error

	reloadMu.Lock()
	defer reloadMu.Unlock()

	err = Load()
	if err != nil {
		return err
	}

	existing = make(map[string]*session)

	shared.mu.RLock()
	for name, current = range shared.sessions {
		existing[name] = current
	}
	shared.mu.RUnlock()

	reused = make(map[string]bool)
	dialed = make(map[string]*session)

	for index = range Loaded.Servers {
		name = Loaded.Servers[index].Name
		if !serverEnabled(&Loaded.Servers[index]) {
			continue
		}

		order = append(order, name)

		current = existing[name]
		if current != nil && current.client != nil &&
			fingerprint(&current.entry) == fingerprint(&Loaded.Servers[index]) {
			reused[name] = true
			continue
		}

		current = dial(ctx, Loaded.Servers[index])
		if current.err != nil {
			util.Log.Error("mcp server unavailable", "server", name, "error", current.err)
		}

		dialed[name] = current
	}

	shared.mu.Lock()
	shared.sessions = make(map[string]*session)

	for _, name = range order {
		if reused[name] {
			shared.sessions[name] = existing[name]
			continue
		}

		shared.sessions[name] = dialed[name]
	}

	shared.order = order
	shared.rebind()
	shared.mu.Unlock()

	for name, current = range existing {
		if reused[name] || current.client == nil {
			continue
		}

		stale = append(stale, current)
	}

	for _, current = range stale {
		current.client.Close()
	}

	return nil
}

func Init(ctx context.Context) error {
	return Reload(ctx)
}
