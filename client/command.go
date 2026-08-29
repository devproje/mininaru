// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/devproje/mininaru/core"
)

type command struct {
	name  string
	usage string
	short string
	run   func(sh *Shell, args string) error
}

const bashShareLimit int = 8000

var commands map[string]*command = map[string]*command{}

func register(cmd *command) {
	commands[cmd.name] = cmd
}

func init() {
	register(&command{name: "help", short: "show this list", run: cmdHelp})
	register(&command{name: "exit", short: "leave the client", run: cmdExit})
	register(&command{name: "clear", short: "clear the screen", run: cmdClear})
	register(&command{name: "bash", usage: "<command...>", short: "run one shell command", run: cmdBash})
	register(&command{name: "!bash", usage: "<command...>", short: "run one shell command, don't share it with the agent", run: cmdBashQuiet})
	register(&command{name: "session", usage: "[id-or-name]", short: "show or switch session", run: cmdSession})
	register(&command{name: "agent", usage: "<id-or-name>", short: "switch agent on a new session", run: cmdAgent})
	register(&command{name: "model", usage: "<model>", short: "change the agent model", run: cmdModel})
	register(&command{name: "effort", usage: "off|low|medium|high|max", short: "change the thinking level", run: cmdEffort})
	register(&command{name: "yolo", usage: "[off|persist|on]", short: "show or set approval mode for this directory", run: cmdYolo})
}

func dispatch(sh *Shell, line string) error {
	var name string
	var args string
	var cmd *command
	var ok bool

	name, args, _ = strings.Cut(strings.TrimPrefix(line, "/"), " ")
	args = strings.TrimSpace(args)

	cmd, ok = commands[name]
	if !ok {
		return fmt.Errorf("unknown command /%s — try /help", name)
	}

	return cmd.run(sh, args)
}

func cmdHelp(sh *Shell, args string) error {
	var names []string
	var name string
	var cmd *command

	for name = range commands {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name = range names {
		cmd = commands[name]
		write("  %s/%s %s%s%s %s\n", PURPLE, cmd.name, DIM, cmd.usage, RESET, cmd.short)
	}

	write("\n  %s⚠ /bash commands and their output are recorded in the session, so the agent reads them.%s\n", YELLOW, RESET)
	write("  %s  do not run anything that reveals passwords, tokens or keys.%s\n", YELLOW, RESET)
	write("  %s  use /!bash to run without sharing the output with the agent.%s\n", YELLOW, RESET)

	return nil
}

func cmdExit(sh *Shell, args string) error {
	sh.quit = true

	return nil
}

func cmdClear(sh *Shell, args string) error {
	write("\x1b[H\x1b[2J")

	return nil
}

func bashTranscript(args string, out string, runErr error) string {
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

type crlfWriter struct {
	out io.Writer
}

func (w crlfWriter) Write(p []byte) (int, error) {
	var err error

	_, err = w.out.Write([]byte(strings.ReplaceAll(string(p), "\n", "\r\n")))
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func feedChild(stream keys, stdin io.WriteCloser, cmd *exec.Cmd, done chan struct{}) {
	var b byte
	var ok bool

	defer stdin.Close()

	for {
		select {
		case <-done:
			return
		case b, ok = <-stream:
			if !ok {
				return
			}

			if b == 0x03 {
				killGroup(cmd)

				continue
			}

			stdin.Write([]byte{b})
		}
	}
}

func cmdBash(sh *Shell, args string) error {
	return runBash(sh, args, true)
}

func cmdBashQuiet(sh *Shell, args string) error {
	return runBash(sh, args, false)
}

func runBash(sh *Shell, args string, share bool) error {
	var shellPath string
	var cmd *exec.Cmd
	var stdin io.WriteCloser
	var out bytes.Buffer
	var done chan struct{}

	var err error

	if args == "" {
		return fmt.Errorf("usage: /bash <command...>")
	}

	shellPath = os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}

	cmd = exec.Command(shellPath, "-c", args)
	cmd.Dir = sh.cwd
	cmd.Stdout = crlfWriter{out: io.MultiWriter(os.Stdout, &out)}
	cmd.Stderr = crlfWriter{out: io.MultiWriter(os.Stderr, &out)}
	setGroup(cmd)

	stdin, err = cmd.StdinPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	done = make(chan struct{})
	go feedChild(sh.keys, stdin, cmd, done)

	err = cmd.Wait()
	close(done)

	if err != nil {
		write("%s✗ %s%s\n", RED, err, RESET)
	}

	if !share {
		return nil
	}

	return Api(http.MethodPost, sh.base+"/sessions/"+sh.session.Id+"/messages", sh.apiKey,
		map[string]string{"role": "user", "content": bashTranscript(args, out.String(), err)}, nil)
}

func cmdSession(sh *Shell, args string) error {
	var found *core.Session

	var err error

	if args == "" {
		write("  %ssession%s %s %s(%s)%s\n", GRAY, RESET, sh.session.Id, DIM, sh.session.Name, RESET)

		return nil
	}

	found, err = sh.findSession(args)
	if err != nil {
		return err
	}

	sh.session = found

	return sh.attach()
}

func cmdAgent(sh *Shell, args string) error {
	var target *core.Agent
	var created core.Session

	var err error

	if args == "" {
		write("  %sagent%s %s\n", GRAY, RESET, sh.agent.Name)

		return nil
	}

	target, err = Agent(sh.base, sh.apiKey, args)
	if err != nil {
		return err
	}

	err = Api(http.MethodPost, sh.base+"/sessions", sh.apiKey, map[string]string{"agent_id": target.Id}, &created)
	if err != nil {
		return err
	}

	sh.agent = target
	sh.session = &created

	return sh.attach()
}

func cmdModel(sh *Shell, args string) error {
	if args == "" {
		write("  %smodel%s %s\n", GRAY, RESET, sh.agent.Model)

		return nil
	}

	return sh.patchAgent(map[string]string{"model": args})
}

func cmdEffort(sh *Shell, args string) error {
	if args == "" {
		write("  %seffort%s %s\n", GRAY, RESET, sh.agent.ThinkingLevel)

		return nil
	}

	switch args {
	case "off", "low", "medium", "high", "max":
	default:
		return fmt.Errorf("usage: /effort off|low|medium|high|max")
	}

	return sh.patchAgent(map[string]string{"thinking_level": args})
}

func cmdYolo(sh *Shell, args string) error {
	var reply struct {
		Root string `json:"root"`
		Mode string `json:"mode"`
	}

	var err error

	if args == "" {
		err = Api(http.MethodGet, fmt.Sprintf("%s/yolo?cwd=%s", sh.base, url.QueryEscape(sh.cwd)), sh.apiKey, nil, &reply)
		if err != nil {
			return err
		}

		write("  %syolo%s %s %s(%s)%s\n", GRAY, RESET, reply.Mode, DIM, reply.Root, RESET)
		sh.yolo = reply.Mode

		return nil
	}

	switch args {
	case "off", "persist", "on":
	default:
		return fmt.Errorf("usage: /yolo off|persist|on")
	}

	err = Api(http.MethodPost, sh.base+"/yolo", sh.apiKey, map[string]string{"mode": args, "cwd": sh.cwd}, &reply)
	if err != nil {
		return err
	}

	sh.yolo = reply.Mode
	write("  %syolo%s %s %s(%s)%s\n", GRAY, RESET, reply.Mode, DIM, reply.Root, RESET)

	return nil
}
