// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

type Options struct {
	Url     string
	Session string
	Agent   string
	ApiKey  string
}

type Shell struct {
	url     string
	base    string
	apiKey  string
	cwd     string
	yolo    string
	conn    *websocket.Conn
	agent   *core.Agent
	session *core.Session
	history []string
	keys    keys
	frames  <-chan Reply
	inbox   ambient
	state   *term.State
	quit    bool
}

func (sh *Shell) patchAgent(payload map[string]string) error {
	var updated core.Agent

	var err error

	err = Api(http.MethodPatch, sh.base+"/agents/"+sh.agent.Id, sh.apiKey, payload, &updated)
	if err != nil {
		return err
	}

	sh.agent = &updated

	return nil
}

func (sh *Shell) findSession(ref string) (*core.Session, error) {
	var list []*core.Session
	var item *core.Session
	var found core.Session

	var err error

	err = Api(http.MethodGet, sh.base+"/sessions/"+ref, sh.apiKey, nil, &found)
	if err == nil {
		return &found, nil
	}

	err = Api(http.MethodGet, fmt.Sprintf("%s/sessions?agent_id=%s", sh.base, url.QueryEscape(sh.agent.Id)), sh.apiKey, nil, &list)
	if err != nil {
		return nil, err
	}

	for _, item = range list {
		if item.Name == ref {
			return item, nil
		}
	}

	return nil, fmt.Errorf("no session %q", ref)
}

func (sh *Shell) attach() error {
	return sh.conn.WriteJSON(Frame{Type: "attach", SessionId: sh.session.Id})
}

func (sh *Shell) reconnect() error {
	var conn *websocket.Conn

	var err error

	conn, err = Dial(sh.url, sh.apiKey)
	if err != nil {
		return err
	}

	if sh.conn != nil {
		sh.conn.Close()
	}

	sh.conn = conn

	err = sh.attach()
	if err != nil {
		return err
	}

	sh.frames = Pump(sh.conn)

	return nil
}

func (sh *Shell) raw() error {
	var state *term.State

	var err error

	state, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}

	sh.state = state

	return nil
}

func (sh *Shell) prompt() string {
	var effort string
	var branch string
	var left string

	if sh.agent.ThinkingLevel != "" {
		effort = fmt.Sprintf(" %s[%s]%s", effortColor(sh.agent.ThinkingLevel), sh.agent.ThinkingLevel, RESET)
	}

	branch = gitBranch(sh.cwd)
	if branch != "" {
		branch = fmt.Sprintf("%sgit:(%s)%s", GREEN, branch, RESET)
	}

	if branch != "" {
		branch = fmt.Sprintf(" %s", branch)
	}

	left = fmt.Sprintf("%s%s%s%s %s%s%s%s", PURPLE, sh.agent.Name, RESET, effort, DIM, sh.session.Name, RESET, branch)

	return fmt.Sprintf("%s\n%s%s%s %s❯%s ",
		left,
		pathColor(sh.yolo), shortPath(sh.cwd), RESET,
		PURPLE, RESET)
}

func (sh *Shell) banner() {
	var notice string

	write("\n%s\n\n", util.NaruLogoWithPad("  "))
	write("  %smininaru%s %s%s (%s)%s\n", BOLD, RESET, DIM, util.AppVersion, util.AppHash, RESET)
	write("  %s↑/↓%s history   %sShift+Enter%s newline   %sCtrl+C%s interrupt   %sCtrl+D%s exit   %s/help%s commands\n\n",
		GRAY, RESET, GRAY, RESET, GRAY, RESET, GRAY, RESET, GRAY, RESET)
	write("  %s●%s %s %s%s%s\n", GREEN, RESET, sh.agent.Name, DIM, sh.session.Id, RESET)

	notice = util.UpdateNotice()
	if notice != "" {
		write("  %s↑%s %s\n", YELLOW, RESET, notice)
	}

	write("\n")
}

func (sh *Shell) send(prompt string) error {
	var err error

	err = sh.conn.WriteJSON(Frame{SessionId: sh.session.Id, Content: prompt, Cwd: sh.cwd})
	if err == nil {
		return nil
	}

	err = sh.reconnect()
	if err != nil {
		return err
	}

	return sh.conn.WriteJSON(Frame{SessionId: sh.session.Id, Content: prompt, Cwd: sh.cwd})
}

func (sh *Shell) turn(prompt string) error {
	var err error

	err = sh.send(prompt)
	if err != nil {
		return err
	}

	err = Receive(sh.conn, sh.frames, sh.session.Id, sh.keys, "")
	if errors.Is(err, errGone) {
		sh.reconnect()
	}

	return err
}

func (sh *Shell) handle(line string) {
	var err error

	if strings.HasPrefix(line, "/") {
		err = dispatch(sh, line)
		if err != nil {
			write("%s✗ %s%s\n", RED, err, RESET)
		}

		return
	}

	err = sh.turn(line)
	if err != nil {
		write("%s✗ %s%s\n", RED, err, RESET)
	}
}

func (sh *Shell) ambientBlock(reply Reply) string {
	sh.inbox.feed(reply)

	if reply.Type != "done" && reply.Type != "error" {
		return ""
	}

	return sh.inbox.flush()
}

func (sh *Shell) loop() error {
	var line string
	var input editor

	var err error

	input = editor{keys: sh.keys, onFrame: sh.ambientBlock, history: sh.history}

	for !sh.quit {
		input.prompt = sh.prompt()
		input.history = sh.history
		input.frames = sh.frames

		line, err = input.readLine()
		if errors.Is(err, errGone) {
			err = sh.reconnect()
			if err != nil {
				return err
			}

			continue
		}
		if errors.Is(err, errInterrupted) {
			continue
		}
		if errors.Is(err, io.EOF) {
			write("\n")

			return nil
		}
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		sh.history = recordHistory(sh.history, line)
		sh.handle(line)
	}

	return nil
}

func Run(opts Options) error {
	var sh Shell

	var err error

	if !isTty() {
		return fmt.Errorf("stdin is not a terminal — use -p for a one-shot prompt")
	}

	sh = Shell{url: opts.Url, apiKey: ResolveApiKey(opts.ApiKey, opts.Url)}

	sh.cwd, err = os.Getwd()
	if err != nil {
		return err
	}

	sh.base, err = ApiBase(opts.Url)
	if err != nil {
		return err
	}

	sh.session, err = Session(sh.base, sh.apiKey, opts.Session, opts.Agent)
	if err != nil {
		return err
	}

	sh.agent, err = Agent(sh.base, sh.apiKey, sh.session.AgentId)
	if err != nil {
		return err
	}

	sh.conn, err = Dial(opts.Url, sh.apiKey)
	if err != nil {
		return err
	}
	defer sh.conn.Close()

	err = sh.attach()
	if err != nil {
		return err
	}

	sh.history = loadHistory()
	sh.banner()

	cmdYolo(&sh, "")

	err = sh.raw()
	if err != nil {
		return err
	}

	write(kittyKbEnable)

	defer func() {
		write(kittyKbDisable)
		term.Restore(int(os.Stdin.Fd()), sh.state)
	}()

	sh.keys = newKeys()
	sh.frames = Pump(sh.conn)

	err = sh.loop()

	saveHistory(sh.history)

	return err
}
