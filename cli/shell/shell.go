// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

type mode int

type Options struct {
	Url     string
	Session string
	Agent   string
}

type state struct {
	mode    mode
	url     string
	seed    string
	agent   string
	name    string
	user    string
	root    bool
	cwd     string
	conn    *websocket.Conn
	session string
}

const (
	MODE_BASH mode = iota
	MODE_AGENT
)

const (
	DEFAULT_URL  string        = "ws://127.0.0.1:8223/ws"
	DIAL_TIMEOUT time.Duration = 3 * time.Second
	SPINNER_TICK time.Duration = 90 * time.Millisecond
)

func dispatch(sh *state, line string, restore func() error, raw func() error) error {
	var args []string
	var nested *exec.Cmd
	var target string

	var err error

	args = strings.Fields(line)

	if sh.mode == MODE_AGENT {
		err = sendAgent(sh, line)
		if err != nil {
			disconnect(sh, err)
		}

		return nil
	}

	if args[0] == "exit" || args[0] == "quit" {
		return io.EOF
	}

	if args[0] == "cd" {
		changeDir(sh, args)
		return nil
	}

	nested, target = escalate(sh, args)

	err = restore()
	if err != nil {
		return err
	}

	if nested != nil {
		runNested(nested, target)
	} else {
		runBash(sh, line)
	}

	return raw()
}

func Run(opts Options) error {
	var fd int
	var sh state
	var previous *term.State
	var line string

	var err error

	fd = int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("shell requires a terminal")
	}

	sh.cwd, err = os.Getwd()
	if err != nil {
		return err
	}

	sh.url = opts.Url
	sh.seed = opts.Session
	sh.agent = opts.Agent
	sh.mode = MODE_BASH
	sh.root = os.Geteuid() == 0
	sh.user = currentUser()

	err = connect(&sh)
	if err != nil {
		util.Log.Debug("shell websocket unavailable", "error", err)
	}

	previous, err = term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, previous)

	if sh.conn != nil {
		defer sh.conn.Close()
	}

	banner(&sh)

	for {
		line, err = readLine(&sh)
		if err != nil {
			write("\n")
			break
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		err = dispatch(&sh, line, func() error {
			return term.Restore(fd, previous)
		}, func() error {
			_, err = term.MakeRaw(fd)
			return err
		})
		if err != nil {
			break
		}
	}

	return nil
}
