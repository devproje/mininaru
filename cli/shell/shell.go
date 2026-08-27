// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	ApiKey  string
}

type state struct {
	mode          mode
	url           string
	seed          string
	agent         string
	apiKey        string
	name          string
	thinkingLevel string
	user          string
	root          bool
	cwd           string
	yoloMode      string
	conn          *websocket.Conn
	connMu        sync.Mutex
	session       string
	history       []string
	agentHistory  []string
	continuation  bool
	mirror        *renderState
	frames        chan inbound
	dial          chan dialResult
	retryAt       time.Time
	retryDelay    time.Duration
	wasAgent      bool
	pendingInput  []byte
	killBuffer    []rune
	gitBranch     string
	lastExitCode  int
}

const (
	MODE_BASH mode = iota
	MODE_AGENT
)

const (
	DEFAULT_URL  string        = "ws://127.0.0.1:8223/ws"
	DIAL_TIMEOUT time.Duration = 3 * time.Second
	SPINNER_TICK time.Duration = 90 * time.Millisecond
	RETRY_MIN    time.Duration = time.Second
	RETRY_MAX    time.Duration = 30 * time.Second
)

func dispatch(sh *state, line string, restore func() error, raw func() error) error {
	var args []string
	var nested *exec.Cmd
	var target string

	var err error

	args = strings.Fields(line)

	if sh.mode == MODE_AGENT {
		if isCommand(line) {
			return dispatchCommand(sh, line)
		}

		err = sendAgent(sh, expandFileReferences(sh, line))
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

	if args[0] == "history" {
		printHistory(sh, args)
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
	var composing string
	var prefs *preferences

	var err error

	fd = int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("shell requires a terminal")
	}

	sh.cwd, err = os.Getwd()
	if err != nil {
		return err
	}

	refreshGitBranch(&sh)

	sh.url = opts.Url
	sh.seed = opts.Session
	sh.agent = opts.Agent
	sh.apiKey = opts.ApiKey

	if sh.agent == "" {
		prefs, err = loadPreferences()
		if err == nil {
			sh.agent = prefs.Agent
		}
	}

	sh.mode = MODE_BASH
	sh.root = os.Geteuid() == 0
	sh.user = currentUser()

	loadHistory(&sh)
	defer saveHistory(&sh)

	err = connect(&sh)
	if err != nil {
		util.Log.Debug("shell websocket unavailable", "error", err)
	}

	refreshYoloMode(&sh)

	previous, err = term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, previous)

	enableAnsi()

	if sh.conn != nil {
		defer sh.conn.Close()
	}

	banner(&sh)

	for {
		line, err = readLine(&sh)

		if errors.Is(err, errSoftNewline) {
			composing = composing + line + "\n"
			sh.continuation = true
			continue
		}

		if errors.Is(err, errContinuationCanceled) {
			composing = ""
			sh.continuation = false
			continue
		}

		sh.continuation = false

		if err != nil {
			write("\n")
			break
		}

		line = composing + line
		composing = ""

		if strings.TrimSpace(line) == "" {
			continue
		}

		if sh.mode == MODE_BASH {
			line, err = continueLine(&sh, line)
			if errors.Is(err, io.EOF) {
				write("\n")
				break
			}

			if err != nil {
				continue
			}
		}

		recordHistory(&sh, line)

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
