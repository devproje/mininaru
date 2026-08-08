package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/devproje/mininaru/util"
)

type MCPServer struct {
	Name           string            `json:"name"`
	Transport      string            `json:"transport"`
	Enabled        *bool             `json:"enabled,omitempty"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Dir            string            `json:"dir,omitempty"`
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Daemon         *bool             `json:"daemon,omitempty"`
	Permission     string            `json:"permission,omitempty"`
	ToolPermission map[string]string `json:"tool_permission,omitempty"`
}

type MCPConfig struct {
	Servers []MCPServer `json:"servers"`
}

const MCP_PATH = "mcp.json"

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

const builtinServerName = "builtin"

const defaultDialTimeout = 10

var MCP MCPConfig

var serverNamePattern *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

func serverEnabled(entry *MCPServer) bool {
	return entry.Enabled == nil || *entry.Enabled
}

func serverDaemon(entry *MCPServer) bool {
	return entry.Daemon == nil || *entry.Daemon
}

func MCPValidate(entry *MCPServer) error {
	if !serverNamePattern.MatchString(entry.Name) {
		return fmt.Errorf("invalid server name %q", entry.Name)
	}
	if entry.Name == builtinServerName {
		return fmt.Errorf("server name %q is reserved", entry.Name)
	}

	switch entry.Transport {
	case TransportStdio:
		if entry.Command == "" {
			return fmt.Errorf("server %q needs a command", entry.Name)
		}
	case TransportHTTP:
		if !strings.HasPrefix(entry.URL, "http://") && !strings.HasPrefix(entry.URL, "https://") {
			return fmt.Errorf("server %q needs an http or https url", entry.Name)
		}
	default:
		return fmt.Errorf("server %q has unknown transport %q", entry.Name, entry.Transport)
	}

	return nil
}

func mcpAccept(loaded MCPConfig) MCPConfig {
	var seen map[string]bool
	var index int
	var accepted MCPConfig

	var err error

	seen = make(map[string]bool)

	for index = range loaded.Servers {
		err = MCPValidate(&loaded.Servers[index])
		if err != nil {
			util.Log.Warn("ignoring an invalid mcp server entry", "config", MCP_PATH, "error", err)
			continue
		}
		if seen[loaded.Servers[index].Name] {
			util.Log.Warn("ignoring a duplicate mcp server", "config", MCP_PATH, "server", loaded.Servers[index].Name)
			continue
		}

		seen[loaded.Servers[index].Name] = true
		accepted.Servers = append(accepted.Servers, loaded.Servers[index])
	}

	return accepted
}

func MCPSave() error {
	var path string
	var buf []byte

	var err error

	path = util.Path(MCP_PATH)
	buf, err = json.MarshalIndent(MCP, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(path, buf, 0600)
}

func MCPLoad() error {
	var path string
	var buf []byte
	var loaded MCPConfig

	var err error

	MCP = MCPConfig{}

	path = util.Path(MCP_PATH)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		buf, _ = json.MarshalIndent(MCPConfig{Servers: []MCPServer{}}, "", "    ")

		err = util.WriteFileAtomic(path, buf, 0600)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal(buf, &loaded)
	if err != nil {
		return err
	}

	MCP = mcpAccept(loaded)

	return nil
}
