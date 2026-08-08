package modules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/util"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func fakeServer() *mcp.Server {
	var server *mcp.Server
	var reply mcp.ToolHandler

	server = mcp.NewServer(&mcp.Implementation{Name: "fake", Version: "test"}, nil)

	reply = func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: req.Params.Name}}}, nil
	}

	server.AddTool(&mcp.Tool{
		Name:        "read_page",
		Description: "read a page",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, reply)

	server.AddTool(&mcp.Tool{
		Name:        "write_page",
		Description: "write a page",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false},
	}, reply)

	server.AddTool(&mcp.Tool{
		Name:        "mystery",
		Description: "unannotated",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, reply)

	return server
}

func singleToolServer(name string) *mcp.Server {
	var server *mcp.Server

	server = mcp.NewServer(&mcp.Implementation{Name: "fake", Version: "test"}, nil)

	server.AddTool(&mcp.Tool{
		Name:        name,
		Description: name,
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: name}}}, nil
	})

	return server
}

func attachServer(t *testing.T, entry MCPServer, server *mcp.Server) {
	var serverTransport *mcp.InMemoryTransport
	var clientTransport *mcp.InMemoryTransport
	var session *mcp.ClientSession
	var tools []*mcp.Tool

	var err error

	t.Helper()

	serverTransport, clientTransport = mcp.NewInMemoryTransports()

	_, err = server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	session, err = mcp.NewClient(&mcp.Implementation{Name: "mininaru", Version: "test"}, nil).
		Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	tools, err = listAllTools(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}

	manager.attach(&mcpSession{entry: entry, session: session, tools: tools})
	t.Cleanup(MCPClose)
}

func findDef(defs []Def, name string) *Def {
	var index int

	for index = range defs {
		if defs[index].Name != name {
			continue
		}

		return &defs[index]
	}

	return nil
}

func TestManagerQualifiesAndClassifiesTools(t *testing.T) {
	var defs []Def
	var def *Def
	var output string

	var err error

	attachServer(t, MCPServer{Name: "notion", Transport: TransportStdio, Command: "fake"}, fakeServer())

	defs = manager.defs()
	if len(defs) != 3 {
		t.Fatalf("manager exposed %d tools", len(defs))
	}

	def = findDef(defs, "notion__read_page")
	if def == nil || def.Permission != PermissionSafe {
		t.Fatalf("read_page = %#v", def)
	}

	def = findDef(defs, "notion__write_page")
	if def == nil || def.Permission != PermissionDangerous {
		t.Fatalf("write_page = %#v", def)
	}

	def = findDef(defs, "notion__mystery")
	if def == nil || def.Permission != PermissionDangerous {
		t.Fatalf("unannotated tool must be dangerous: %#v", def)
	}

	if ToolSource("notion__read_page") != "notion" {
		t.Fatalf("ToolSource = %q", ToolSource("notion__read_page"))
	}
	if ToolSource("file_read") != builtinServerName {
		t.Fatal("builtin tool lost its source")
	}

	def = findDef(defs, "notion__read_page")
	output, err = def.Execute(context.Background(), "")
	if err != nil || output != "read_page" {
		t.Fatalf("Execute = %q, %v", output, err)
	}
}

func TestManagerOverrideAndDaemonOptOut(t *testing.T) {
	var daemon bool
	var def *Def

	daemon = false

	attachServer(t, MCPServer{
		Name: "notion", Transport: TransportStdio, Command: "fake",
		Daemon: &daemon, ToolPermission: map[string]string{"write_page": "safe"},
	}, fakeServer())

	def = findDef(manager.defs(), "notion__write_page")
	if def == nil || def.Permission != PermissionSafe {
		t.Fatalf("tool override was ignored: %#v", def)
	}

	if findDef(SafeTools(), "notion__write_page") != nil {
		t.Fatal("a daemon opt-out server reached SafeTools")
	}
	if findDef(SafeTools(), "current_time") == nil {
		t.Fatal("builtin safe tools disappeared from SafeTools")
	}
}

func TestManagerDropsCollidingToolNames(t *testing.T) {
	var defs []Def
	var def Def
	var count int

	attachServer(t, MCPServer{Name: "a", Transport: TransportStdio, Command: "fake"}, singleToolServer("b__c"))
	attachServer(t, MCPServer{Name: "a__b", Transport: TransportStdio, Command: "fake"}, singleToolServer("c"))

	defs = manager.defs()

	for _, def = range defs {
		if def.Name != "a__b__c" {
			continue
		}

		count++
	}

	if count != 1 {
		t.Fatalf("colliding tool name survived %d times", count)
	}
	if ToolSource("a__b__c") != "a" {
		t.Fatalf("first registration lost the name: %q", ToolSource("a__b__c"))
	}
}

func TestMCPInitSurvivesUnreachableServer(t *testing.T) {
	var status []MCPStatus

	var err error

	util.RootDir = t.TempDir()

	err = MCPLoad()
	if err != nil {
		t.Fatal(err)
	}

	MCP.Servers = []MCPServer{
		{Name: "ghost", Transport: TransportStdio, Command: "mininaru-no-such-binary", TimeoutSeconds: 2},
	}

	err = MCPSave()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(MCPClose)

	err = MCPInit(context.Background())
	if err != nil {
		t.Fatalf("MCPInit failed on an unreachable server: %v", err)
	}

	status = MCPStatusAll()
	if len(status) != 1 || status[0].Connected || status[0].Error == "" {
		t.Fatalf("MCPStatusAll = %#v", status)
	}

	if len(DefaultTools()) != len(builtinTools()) {
		t.Fatalf("builtin tools disappeared: %d", len(DefaultTools()))
	}
}

func TestHTTPTransportSendsHeaders(t *testing.T) {
	var seen chan string
	var handler *mcp.StreamableHTTPHandler
	var httpServer *httptest.Server
	var current *mcpSession
	var token string

	seen = make(chan string, 8)

	handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return fakeServer() }, nil)

	httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case seen <- r.Header.Get("X-Mininaru"):
		default:
		}

		handler.ServeHTTP(w, r)
	}))
	defer httpServer.Close()

	current = mcpDial(context.Background(), MCPServer{
		Name: "remote", Transport: TransportHTTP, URL: httpServer.URL,
		Headers: map[string]string{"X-Mininaru": "token"}, TimeoutSeconds: 5,
	})
	if current.err != nil {
		t.Fatal(current.err)
	}

	defer current.session.Close()

	if len(current.tools) != 3 {
		t.Fatalf("http server listed %d tools", len(current.tools))
	}

	token = <-seen
	if token != "token" {
		t.Fatalf("configured header was not sent: %q", token)
	}
}
