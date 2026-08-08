package modules

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestMCPLoadCreatesEmptyConfig(t *testing.T) {
	var info os.FileInfo

	var err error

	util.RootDir = t.TempDir()

	err = MCPLoad()
	if err != nil {
		t.Fatal(err)
	}

	if len(MCP.Servers) != 0 {
		t.Fatalf("fresh config listed %d servers", len(MCP.Servers))
	}

	info, err = os.Stat(util.Path(MCP_PATH))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("%s mode = %v", MCP_PATH, info.Mode().Perm())
	}
}

func TestMCPValidate(t *testing.T) {
	var cases []MCPServer
	var index int
	var entry MCPServer

	cases = []MCPServer{
		{Name: "", Transport: TransportStdio, Command: "echo"},
		{Name: "bad name", Transport: TransportStdio, Command: "echo"},
		{Name: builtinServerName, Transport: TransportStdio, Command: "echo"},
		{Name: "ok", Transport: "carrier-pigeon", Command: "echo"},
		{Name: "ok", Transport: TransportStdio},
		{Name: "ok", Transport: TransportHTTP},
		{Name: "ok", Transport: TransportHTTP, URL: "ftp://example.com"},
	}

	for index = range cases {
		if MCPValidate(&cases[index]) == nil {
			t.Fatalf("case %d was accepted: %#v", index, cases[index])
		}
	}

	entry = MCPServer{Name: "notion", Transport: TransportHTTP, URL: "https://example.com/mcp"}
	if MCPValidate(&entry) != nil {
		t.Fatal("valid http entry was rejected")
	}

	entry = MCPServer{Name: "local_fs-1", Transport: TransportStdio, Command: "echo"}
	if MCPValidate(&entry) != nil {
		t.Fatal("valid stdio entry was rejected")
	}
}

func TestMCPSaveRoundTrip(t *testing.T) {
	var enabled bool

	var err error

	util.RootDir = t.TempDir()

	err = MCPLoad()
	if err != nil {
		t.Fatal(err)
	}

	MCP.Servers = []MCPServer{
		{Name: "good", Transport: TransportStdio, Command: "echo", Args: []string{"hi"}},
		{Name: "broken", Transport: TransportStdio},
		{Name: "good", Transport: TransportStdio, Command: "echo"},
	}

	err = MCPSave()
	if err != nil {
		t.Fatal(err)
	}

	err = MCPLoad()
	if err != nil {
		t.Fatal(err)
	}

	if len(MCP.Servers) != 1 || MCP.Servers[0].Name != "good" {
		t.Fatalf("reload kept %#v", MCP.Servers)
	}
	if !serverEnabled(&MCP.Servers[0]) || !serverDaemon(&MCP.Servers[0]) {
		t.Fatal("absent enabled and daemon flags must default to true")
	}

	enabled = false
	MCP.Servers[0].Enabled = &enabled

	err = MCPSave()
	if err != nil {
		t.Fatal(err)
	}

	err = MCPLoad()
	if err != nil {
		t.Fatal(err)
	}

	if serverEnabled(&MCP.Servers[0]) {
		t.Fatal("explicit disable did not survive a round trip")
	}
}
