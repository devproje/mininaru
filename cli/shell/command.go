// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devproje/mininaru/core"
)

type commandResult struct {
	Message     string
	Exit        bool
	ClearScreen bool
}

type commandHandler func(sh *state, args []string) (commandResult, error)

type commandEntry struct {
	Name        string
	Description string
	Handler     commandHandler
}

var commands map[string]commandEntry

func registerCommand(name string, description string, handler commandHandler) {
	if commands == nil {
		commands = make(map[string]commandEntry)
	}

	commands[name] = commandEntry{Name: name, Description: description, Handler: handler}
}

func isCommand(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "/")
}

func commandNames() []string {
	var names []string
	var name string

	for name = range commands {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func commandHelp() string {
	var names []string
	var name string
	var builder strings.Builder

	names = commandNames()

	builder.WriteString("available commands:\n")
	for _, name = range names {
		builder.WriteString(fmt.Sprintf("  /%-10s %s\n", name, commands[name].Description))
	}

	return strings.TrimRight(builder.String(), "\n")
}

func resetSessionCommand(sh *state, args []string) (commandResult, error) {
	var base string
	var current core.Session
	var created core.Session

	var err error

	base, err = apiBase(sh.url)
	if err != nil {
		return commandResult{}, err
	}

	err = apiGet(base+"/sessions/"+sh.session, sh.apiKey, &current)
	if err != nil {
		return commandResult{}, err
	}

	err = apiPost(base+"/sessions", sh.apiKey, map[string]string{"agent_id": current.AgentId, "name": "shell"}, &created)
	if err != nil {
		return commandResult{}, err
	}

	sh.session = created.Id

	return commandResult{Message: "started a new session " + created.Id}, nil
}

func showSessionCommand(sh *state, args []string) (commandResult, error) {
	var base string
	var session core.Session

	var err error

	base, err = apiBase(sh.url)
	if err != nil {
		return commandResult{}, err
	}

	err = apiGet(base+"/sessions/"+sh.session, sh.apiKey, &session)
	if err != nil {
		return commandResult{}, err
	}

	return commandResult{Message: fmt.Sprintf("session %s\n  agent   %s\n  created %s", session.Id, agentLabel(sh), session.CreatedAt)}, nil
}

func clearScreenCommand(sh *state, args []string) (commandResult, error) {
	return commandResult{ClearScreen: true}, nil
}

func leaveAgentCommand(sh *state, args []string) (commandResult, error) {
	return commandResult{Exit: true}, nil
}

func helpCommand(sh *state, args []string) (commandResult, error) {
	return commandResult{Message: commandHelp()}, nil
}

func yoloCommand(sh *state, args []string) (commandResult, error) {
	var mode string
	var base string
	var resp map[string]any

	var err error

	if len(args) == 0 {
		return commandResult{}, fmt.Errorf("usage: /yolo <off|persist|on>")
	}

	mode = strings.ToLower(args[0])
	if mode != "off" && mode != "persist" && mode != "on" {
		return commandResult{}, fmt.Errorf("mode must be one of off, persist, on")
	}

	if mode == "on" {
		if !confirmPrompt("yolo on lets dangerous tools run with no approval prompt for " + sh.cwd + " — are you sure?") {
			return commandResult{Message: "cancelled"}, nil
		}
	}

	base, err = apiBase(sh.url)
	if err != nil {
		return commandResult{}, err
	}

	err = apiPost(base+"/yolo", sh.apiKey, map[string]string{"mode": mode, "cwd": sh.cwd}, &resp)
	if err != nil {
		return commandResult{}, err
	}

	sh.yoloMode = mode

	return commandResult{Message: fmt.Sprintf("yolo mode set to %s for %v", mode, resp["root"])}, nil
}

func init() {
	registerCommand("help", "list available commands", helpCommand)
	registerCommand("reset", "start a fresh session with the same agent", resetSessionCommand)
	registerCommand("session", "show the current session id and agent", showSessionCommand)
	registerCommand("clear", "clear the terminal screen", clearScreenCommand)
	registerCommand("exit", "leave agent mode, back to bash", leaveAgentCommand)
	registerCommand("bash", "leave agent mode, back to bash", leaveAgentCommand)
	registerCommand("yolo", "set dangerous-tool trust for this directory (off|persist|on)", yoloCommand)
}

func dispatchCommand(sh *state, line string) error {
	var fields []string
	var name string
	var found commandEntry
	var ok bool
	var result commandResult

	var err error

	fields = strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return nil
	}

	name = strings.ToLower(strings.TrimPrefix(fields[0], "/"))

	found, ok = commands[name]
	if !ok {
		notice(RED, "✖", "unknown command %q, try /help", name)
		return nil
	}

	result, err = found.Handler(sh, fields[1:])
	if err != nil {
		notice(RED, "✖", "%s", err.Error())
		return nil
	}

	if result.ClearScreen {
		write("\x1b[2J\x1b[H")
	}

	if result.Message != "" {
		notice(GRAY, "›", "%s", result.Message)
	}

	if result.Exit {
		sh.mode = MODE_BASH
	}

	return nil
}
