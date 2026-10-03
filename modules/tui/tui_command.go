// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/client"
)

type tuiCmdInfo struct {
	name string
	desc string
}

type providerModelsEntry struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type yoloReply struct {
	Root string `json:"root"`
	Mode string `json:"mode"`
}

const bashShareLimit int = 8000

var tuiCmdList = []tuiCmdInfo{
	{"help", "this list"},
	{"clear", "clear the transcript"},
	{"usage", "show context window usage"},
	{"compact", "summarize completed turns"},
	{"session", "show or switch session (by id or name)"},
	{"gateway", "list or switch remote endpoints"},
	{"model", "show available models or set the agent's model"},
	{"effort", "change the thinking level"},
	{"yolo", "show or set approval mode for this directory"},
	{"bash", "run one shell command, share output with the agent"},
	{"!bash", "run one shell command, don't share it with the agent"},
	{"exit", "leave the client"},
}

func tuiHelpLines() []string {
	var lines []string
	var c tuiCmdInfo

	lines = append(lines, fmt.Sprintf("%savailable commands%s", client.PURPLE, client.RESET))

	for _, c = range tuiCmdList {
		lines = append(lines, fmt.Sprintf("  %s/%s%s  %s", client.PURPLE, c.name, client.RESET, c.desc))
	}

	return lines
}

func tuiCmdSuggestions(input string) []tuiCmdInfo {
	var typed string
	var out []tuiCmdInfo
	var c tuiCmdInfo

	if !strings.HasPrefix(input, "/") {
		return nil
	}

	typed, _, _ = strings.Cut(strings.TrimPrefix(input, "/"), " ")
	typed = strings.ToLower(typed)

	for _, c = range tuiCmdList {
		if strings.HasPrefix(c.name, typed) {
			out = append(out, c)
		}
	}

	return out
}

func tuiCmdExact(input string) bool {
	var typed string
	var c tuiCmdInfo

	typed, _, _ = strings.Cut(strings.TrimPrefix(input, "/"), " ")

	for _, c = range tuiCmdList {
		if c.name == typed {
			return true
		}
	}

	return false
}

func patchAgent(base, apiKey, agentId string, payload map[string]string) error {
	var updated core.Agent

	var err error

	err = client.Api(http.MethodPatch, base+"/agents/"+agentId, apiKey, payload, &updated)

	return err
}

func bashTranscript(args, out string, runErr error) string {
	var status string

	status = "ok"
	if runErr != nil {
		status = runErr.Error()
	}

	out = strings.TrimRight(out, "\n")
	if len(out) > bashShareLimit {
		out = fmt.Sprintf("%s\n… truncated", out[:bashShareLimit])
	}

	if out == "" {
		out = "(no output)"
	}

	return fmt.Sprintf("[/bash] %s\n[exit] %s\n%s", args, status, out)
}

func runLocalBash(cwd, args string) (string, error) {
	var shellPath string
	var cmd *exec.Cmd
	var out []byte

	var err error

	shellPath = os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}

	cmd = exec.Command(shellPath, "-c", args)
	cmd.Dir = cwd

	out, err = cmd.CombinedOutput()

	return string(out), err
}

func (m tuiModel) runCommand(line string) (tea.Model, tea.Cmd) {
	var name string
	var args string
	var base string
	var apiKey string
	var session string
	var cmd tea.Cmd
	var gw client.Gateway
	var matched bool
	var target client.Gateway
	var agentId string
	var cwd string

	name, args, _ = strings.Cut(strings.TrimPrefix(line, "/"), " ")
	args = strings.TrimSpace(args)

	m.lines = append(m.lines, tuiUserPrefix.Render("> "+line))

	switch name {
	case "help":
		m.lines = append(m.lines, tuiHelpLines()...)
	case "clear":
		m.lines = []string{tuiHeaderBlock(m.agent, m.session, m.width)}
	case "exit", "quit":
		return m, tea.Quit
	case "usage":
		if m.base == "" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ no session connected%s", client.RED, client.RESET))
			break
		}

		base, apiKey, session = m.base, m.apiKey, m.session
		cmd = m.beginAsync("usage")
		m.layout()

		return m, tea.Batch(cmd, asyncCmd("usage", func() []string {
			var usage core.ContextUsage
			var err error

			err = client.Api(http.MethodGet, base+"/sessions/"+session+"/usage", apiKey, nil, &usage)
			if err != nil {
				return []string{fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)}
			}

			return []string{fmt.Sprintf("%s%s%s", client.GRAY, client.ContextLabel(&usage), client.RESET)}
		}))
	case "compact":
		if m.base == "" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ no session connected%s", client.RED, client.RESET))
			break
		}

		base, apiKey, session = m.base, m.apiKey, m.session
		cmd = m.beginAsync("compact")
		m.layout()

		return m, tea.Batch(cmd, asyncCmd("compact", func() []string {
			var usage core.ContextUsage
			var err error

			err = client.Api(http.MethodPost, base+"/sessions/"+session+"/compact", apiKey, nil, &usage)
			if err != nil {
				return []string{fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)}
			}

			return []string{fmt.Sprintf("%scompacted%s  %s%s%s", client.GRAY, client.RESET, client.GRAY, client.ContextLabel(&usage), client.RESET)}
		}))
	case "session":
		if args == "" {
			m.lines = append(m.lines, fmt.Sprintf("%ssession%s %s", client.GRAY, client.RESET, m.session))
			break
		}

		if m.base == "" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ no session connected%s", client.RED, client.RESET))
			break
		}

		cmd = m.beginAsync("session")
		m.layout()

		return m, tea.Batch(cmd, findSessionCmd(m.base, m.apiKey, m.agentId, args))
	case "gateway":
		if len(m.gateways) == 0 {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ no gateways — add one with 'mininaru gateway add'%s", client.RED, client.RESET))
			break
		}

		if args == "" {
			m.lines = append(m.lines, fmt.Sprintf("%savailable gateways%s", client.PURPLE, client.RESET))
			for _, gw = range m.gateways {
				m.lines = append(m.lines, fmt.Sprintf("  %s%s%s  %s%s%s", client.PURPLE, gw.Name, client.RESET, client.DIM, gw.Url, client.RESET))
			}
			m.lines = append(m.lines, fmt.Sprintf("%suse /gateway <name> to switch (starts a new session there)%s", client.DIM, client.RESET))
			break
		}

		matched = false
		for _, gw = range m.gateways {
			if gw.Name == args {
				target = gw
				matched = true

				break
			}
		}

		if !matched {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ unknown gateway %q%s", client.RED, args, client.RESET))
			break
		}

		cmd = m.beginAsync("gateway")
		m.layout()

		return m, tea.Batch(cmd, switchGatewayCmd(target))
	case "model":
		if m.base == "" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ no session connected%s", client.RED, client.RESET))
			break
		}

		if args != "" && !strings.Contains(args, ":") {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ usage: /model <provider>:<model>%s", client.RED, client.RESET))
			break
		}

		base, apiKey, agentId = m.base, m.apiKey, m.agentId
		cmd = m.beginAsync("model")
		m.layout()

		if args != "" {
			return m, tea.Batch(cmd, asyncCmd("model", func() []string {
				var err error

				err = patchAgent(base, apiKey, agentId, map[string]string{"model": args})
				if err != nil {
					return []string{fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)}
				}

				return []string{fmt.Sprintf("%smodel%s %s", client.GRAY, client.RESET, args)}
			}))
		}

		return m, tea.Batch(cmd, asyncCmd("model", func() []string {
			var agent core.Agent
			var models []providerModelsEntry
			var entry providerModelsEntry
			var out []string

			var err error

			err = client.Api(http.MethodGet, base+"/agents/"+agentId, apiKey, nil, &agent)
			if err != nil {
				return []string{fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)}
			}

			out = append(out, fmt.Sprintf("%smodel%s %s", client.GRAY, client.RESET, agent.Model))

			err = client.Api(http.MethodGet, base+"/providers/models", apiKey, nil, &models)
			if err != nil {
				return append(out, fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET))
			}

			for _, entry = range models {
				out = append(out, fmt.Sprintf("  %s%s:%s%s", client.PURPLE, entry.Provider, entry.Model, client.RESET))
			}

			return append(out, fmt.Sprintf("%suse /model <provider:model> to switch%s", client.DIM, client.RESET))
		}))
	case "effort":
		if m.base == "" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ no session connected%s", client.RED, client.RESET))
			break
		}

		if args != "" && args != "off" && args != "low" && args != "medium" && args != "high" && args != "max" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ usage: /effort off|low|medium|high|max%s", client.RED, client.RESET))
			break
		}

		base, apiKey, agentId = m.base, m.apiKey, m.agentId
		cmd = m.beginAsync("effort")
		m.layout()

		if args != "" {
			return m, tea.Batch(cmd, asyncCmd("effort", func() []string {
				var err error

				err = patchAgent(base, apiKey, agentId, map[string]string{"thinking_level": args})
				if err != nil {
					return []string{fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)}
				}

				return []string{fmt.Sprintf("%seffort%s %s", client.GRAY, client.RESET, args)}
			}))
		}

		return m, tea.Batch(cmd, asyncCmd("effort", func() []string {
			var agent core.Agent
			var err error

			err = client.Api(http.MethodGet, base+"/agents/"+agentId, apiKey, nil, &agent)
			if err != nil {
				return []string{fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)}
			}

			return []string{fmt.Sprintf("%seffort%s %s", client.GRAY, client.RESET, agent.ThinkingLevel)}
		}))
	case "yolo":
		if m.base == "" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ no session connected%s", client.RED, client.RESET))
			break
		}

		if args != "" && args != "off" && args != "persist" && args != "on" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ usage: /yolo off|persist|on%s", client.RED, client.RESET))
			break
		}

		base, apiKey, cwd = m.base, m.apiKey, m.cwd
		cmd = m.beginAsync("yolo")
		m.layout()

		return m, tea.Batch(cmd, asyncCmd("yolo", func() []string {
			var yolo yoloReply
			var err error

			if args == "" {
				err = client.Api(http.MethodGet, fmt.Sprintf("%s/yolo?cwd=%s", base, url.QueryEscape(cwd)), apiKey, nil, &yolo)
			} else {
				err = client.Api(http.MethodPost, base+"/yolo", apiKey, map[string]string{"mode": args, "cwd": cwd}, &yolo)
			}

			if err != nil {
				return []string{fmt.Sprintf("%s✗ %s%s", client.RED, err, client.RESET)}
			}

			return []string{fmt.Sprintf("%syolo%s %s %s(%s)%s", client.GRAY, client.RESET, yolo.Mode, client.DIM, yolo.Root, client.RESET)}
		}))
	case "bash", "!bash":
		if args == "" {
			m.lines = append(m.lines, fmt.Sprintf("%s✗ usage: /%s <command...>%s", client.RED, name, client.RESET))
			break
		}

		base, apiKey, cwd, session = m.base, m.apiKey, m.cwd, m.session
		cmd = m.beginAsync(name)
		m.layout()

		return m, tea.Batch(cmd, asyncCmd(name, func() []string {
			var output string
			var runErr error
			var shareErr error
			var out []string

			output, runErr = runLocalBash(cwd, args)

			out = append(out, fmt.Sprintf("%s %s/%s%s finished", client.StatusDot("finished"), client.WHITE, name, client.RESET))

			if strings.TrimSpace(output) == "" {
				out = append(out, fmt.Sprintf("%s(no output)%s", client.DIM, client.RESET))
			} else {
				out = append(out, dimEachLine(strings.TrimRight(output, "\n")))
			}

			if runErr != nil {
				out = append(out, fmt.Sprintf("%s✗ %s%s", client.RED, runErr, client.RESET))
			}

			if name == "bash" && base != "" {
				shareErr = client.Api(http.MethodPost, base+"/sessions/"+session+"/messages", apiKey,
					map[string]string{"role": "user", "content": bashTranscript(args, output, runErr)}, nil)
				if shareErr != nil {
					out = append(out, fmt.Sprintf("%s✗ share failed: %s%s", client.RED, shareErr, client.RESET))
				}
			}

			return out
		}))
	default:
		m.lines = append(m.lines, fmt.Sprintf("%s✗ unknown command /%s — try /help%s", client.RED, name, client.RESET))
	}

	m.newLine()
	m.layout()

	return m, nil
}
