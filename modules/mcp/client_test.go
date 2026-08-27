// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func fakeServer() *mcpsdk.Server {
	var server *mcpsdk.Server
	var reply mcpsdk.ToolHandler

	server = mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake", Version: "test"}, nil)

	reply = func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: req.Params.Name}}}, nil
	}

	server.AddTool(&mcpsdk.Tool{
		Name:        "read_page",
		Description: "read a page",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, reply)

	server.AddTool(&mcpsdk.Tool{
		Name:        "write_page",
		Description: "write a page",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false},
	}, reply)

	return server
}

func attachTestServer(t *testing.T, entry Server, server *mcpsdk.Server) {
	var serverTransport *mcpsdk.InMemoryTransport
	var clientTransport *mcpsdk.InMemoryTransport
	var client *mcpsdk.ClientSession
	var tools []*mcpsdk.Tool

	var err error

	t.Helper()

	serverTransport, clientTransport = mcpsdk.NewInMemoryTransports()

	_, err = server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	client, err = mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mininaru", Version: "test"}, nil).
		Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	tools, err = listAllTools(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}

	shared.mu.Lock()
	shared.sessions[entry.Name] = &session{entry: entry, client: client, tools: tools}
	shared.order = append(shared.order, entry.Name)
	shared.rebind()
	shared.mu.Unlock()

	t.Cleanup(Close)
}

func findTool(tools []modules.Tool, name string) *modules.Tool {
	var index int

	for index = range tools {
		if tools[index].Name == name {
			return &tools[index]
		}
	}

	return nil
}

func TestManagerQualifiesAndClassifiesTools(t *testing.T) {
	var tools []modules.Tool
	var tool *modules.Tool
	var output string

	var err error

	attachTestServer(t, Server{Name: "notion", Transport: TransportStdio, Command: "fake"}, fakeServer())

	tools = Tools()
	if len(tools) != 2 {
		t.Fatalf("manager exposed %d tools", len(tools))
	}

	tool = findTool(tools, "notion__read_page")
	if tool == nil || tool.Permission != modules.PermissionSafe {
		t.Fatalf("read_page = %#v", tool)
	}

	tool = findTool(tools, "notion__write_page")
	if tool == nil || tool.Permission != modules.PermissionDangerous {
		t.Fatalf("write_page = %#v", tool)
	}

	output, err = tool.Execute(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if output != "write_page" {
		t.Fatalf("Execute = %q, want the qualified tool's underlying name", output)
	}
}

func TestStatusAllReportsConnectedDisabledAndFailedServers(t *testing.T) {
	var disabled bool
	var all []Status
	var byName map[string]Status
	var one Status

	disabled = false

	attachTestServer(t, Server{Name: "notion", Transport: TransportStdio, Command: "fake"}, fakeServer())

	shared.mu.Lock()
	shared.sessions["broken"] = &session{entry: Server{Name: "broken", Transport: TransportStdio}, err: fmt.Errorf("dial failed")}
	shared.order = append(shared.order, "broken")
	shared.mu.Unlock()

	Loaded = Config{Servers: []Server{
		{Name: "notion", Transport: TransportStdio},
		{Name: "broken", Transport: TransportStdio},
		{Name: "off", Transport: TransportStdio, Enabled: &disabled},
	}}
	t.Cleanup(func() { Loaded = Config{} })

	all = StatusAll()

	byName = make(map[string]Status, len(all))
	for _, one = range all {
		byName[one.Name] = one
	}

	if !byName["notion"].Connected || byName["notion"].Tools != 2 {
		t.Fatalf("notion = %+v, want connected with 2 tools", byName["notion"])
	}
	if byName["broken"].Connected || byName["broken"].Error == "" {
		t.Fatalf("broken = %+v, want not connected with an error", byName["broken"])
	}
	if byName["off"].Enabled {
		t.Fatalf("off = %+v, want enabled=false", byName["off"])
	}
}

func TestManagerOverridesToolPermission(t *testing.T) {
	var tool *modules.Tool

	attachTestServer(t, Server{
		Name: "notion", Transport: TransportStdio, Command: "fake",
		ToolPermission: map[string]string{"write_page": "safe"},
	}, fakeServer())

	tool = findTool(Tools(), "notion__write_page")
	if tool == nil || tool.Permission != modules.PermissionSafe {
		t.Fatalf("tool override was ignored: %#v", tool)
	}
}

func TestReloadDoesNotRedialAnUnchangedServer(t *testing.T) {
	var dials int
	var handler *mcpsdk.StreamableHTTPHandler
	var upstream *httptest.Server
	var tools []modules.Tool
	var body []byte

	var err error

	handler = mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return fakeServer() }, nil)
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			if bytes.Contains(body, []byte(`"method":"initialize"`)) {
				dials++
			}
		}

		handler.ServeHTTP(w, r)
	}))
	defer upstream.Close()
	defer Close()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	Loaded = Config{Servers: []Server{{Name: "remote", Transport: TransportHTTP, URL: upstream.URL, TimeoutSeconds: 5}}}
	err = Save()
	if err != nil {
		t.Fatal(err)
	}

	err = Reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tools = Tools()
	if len(tools) != 2 {
		t.Fatalf("first reload exposed %d tools, want 2", len(tools))
	}
	if dials != 1 {
		t.Fatalf("dials after first reload = %d, want 1", dials)
	}

	err = Reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dials != 1 {
		t.Fatalf("dials after a reload with an unchanged fingerprint = %d, want 1 (no redial)", dials)
	}

	Loaded = Config{Servers: []Server{{Name: "remote", Transport: TransportHTTP, URL: upstream.URL, TimeoutSeconds: 10}}}
	err = Save()
	if err != nil {
		t.Fatal(err)
	}
	err = Reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dials != 2 {
		t.Fatalf("dials after a config change = %d, want 2 (redial)", dials)
	}
}
