// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/devproje/mininaru/util"
)

type Server struct {
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
	Permission     string            `json:"permission,omitempty"`
	ToolPermission map[string]string `json:"tool_permission,omitempty"`
}

type Config struct {
	Servers []Server `json:"servers"`
}

const configPath = "mcp.json"

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

const defaultDialTimeout = 10

var Loaded Config

var serverNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

func serverEnabled(entry *Server) bool {
	return entry.Enabled == nil || *entry.Enabled
}

func Validate(entry *Server) error {
	if !serverNamePattern.MatchString(entry.Name) {
		return fmt.Errorf("invalid server name %q", entry.Name)
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

func accept(loaded Config) Config {
	var seen map[string]bool
	var index int
	var accepted Config

	var err error

	seen = make(map[string]bool)

	for index = range loaded.Servers {
		err = Validate(&loaded.Servers[index])
		if err != nil {
			util.Log.Warn("ignoring an invalid mcp server entry", "config", configPath, "error", err)
			continue
		}
		if seen[loaded.Servers[index].Name] {
			util.Log.Warn("ignoring a duplicate mcp server", "config", configPath, "server", loaded.Servers[index].Name)
			continue
		}

		seen[loaded.Servers[index].Name] = true
		accepted.Servers = append(accepted.Servers, loaded.Servers[index])
	}

	return accepted
}

func mapHeaders(config Config, transform func(string) (string, error)) (Config, error) {
	var out Config
	var index int
	var name string
	var value string
	var transformed string

	var err error

	out.Servers = make([]Server, len(config.Servers))
	copy(out.Servers, config.Servers)

	for index = range out.Servers {
		if out.Servers[index].Headers == nil {
			continue
		}

		out.Servers[index].Headers = make(map[string]string, len(config.Servers[index].Headers))

		for name, value = range config.Servers[index].Headers {
			transformed, err = transform(value)
			if err != nil {
				return config, err
			}

			out.Servers[index].Headers[name] = transformed
		}
	}

	return out, nil
}

func Save() error {
	var path string
	var buf []byte
	var encrypted Config

	var err error

	path = util.Path(configPath)

	encrypted, err = mapHeaders(Loaded, util.Encrypt)
	if err != nil {
		return err
	}

	buf, err = json.MarshalIndent(encrypted, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(path, buf, 0600)
}

func Load() error {
	var path string
	var buf []byte
	var loaded Config

	var err error

	Loaded = Config{}

	path = util.Path(configPath)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		buf, _ = json.MarshalIndent(Config{Servers: []Server{}}, "", "    ")

		err = util.WriteFileAtomic(path, buf, 0600)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal(buf, &loaded)
	if err != nil {
		return err
	}

	loaded, err = mapHeaders(loaded, util.Decrypt)
	if err != nil {
		return err
	}

	Loaded = accept(loaded)

	return nil
}
