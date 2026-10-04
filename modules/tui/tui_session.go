// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"net/http"
	"net/url"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/client"
	"github.com/gorilla/websocket"
)

type modeReply struct {
	Root string `json:"root"`
	Mode string `json:"mode"`
}

type tuiModeSyncedMsg struct {
	mode string
	err  error
}

type tuiChunkMsg struct {
	reasoning string
	content   string
}

type tuiToolMsg struct {
	name    string
	status  string
	message string
}

type tuiMessageMsg struct {
	name    string
	message string
}

type tuiPromptMsg struct {
	kind      string
	sessionId string
	cwd       string
	name      string
	arguments string
	question  string
	options   []string
}

type tuiDoneMsg struct {
	errText string
}

type tuiAnsweredMsg struct{}

type tuiCmdResultMsg struct {
	label string
	lines []string
}

type tuiSessionSwitchMsg struct {
	found *core.Session
	err   error
}

type tuiGatewaySwitchMsg struct {
	targetName string
	conn       *websocket.Conn
	base       string
	apiKey     string
	agentName  string
	agentId    string
	sessionId  string
	err        error
}

type tuiImgUploadedMsg struct {
	path string
	id   string
	err  error
}

func fetchMode(base, apiKey, cwd string) string {
	var reply modeReply
	var err error

	err = client.Api(http.MethodGet, fmt.Sprintf("%s/mode?cwd=%s", base, url.QueryEscape(cwd)), apiKey, nil, &reply)
	if err != nil {
		return core.ModeDefault
	}

	return reply.Mode
}

func pushMode(base, apiKey, cwd, mode string) (string, error) {
	var reply modeReply

	var err error

	err = client.Api(http.MethodPost, base+"/mode", apiKey, map[string]string{"mode": mode, "cwd": cwd}, &reply)
	if err != nil {
		return "", err
	}

	return reply.Mode, nil
}

func pushModeCmd(base, apiKey, cwd, mode string) tea.Cmd {
	return func() tea.Msg {
		var confirmed string

		var err error

		confirmed, err = pushMode(base, apiKey, cwd, mode)

		return tuiModeSyncedMsg{mode: confirmed, err: err}
	}
}

func refreshModeCmd(base, apiKey, cwd string) tea.Cmd {
	return func() tea.Msg {
		return tuiModeSyncedMsg{mode: fetchMode(base, apiKey, cwd)}
	}
}

func nextMode(mode string) string {
	switch mode {
	case core.ModeDefault:
		return core.ModePlan
	case core.ModePlan:
		return core.ModeAutoPersist
	case core.ModeAutoPersist:
		return core.ModeFullAuto
	default:
		return core.ModeDefault
	}
}

func asyncCmd(label string, fn func() []string) tea.Cmd {
	return func() tea.Msg {
		return tuiCmdResultMsg{label: label, lines: fn()}
	}
}

func findSessionCmd(base, apiKey, agentId, ref string) tea.Cmd {
	return func() tea.Msg {
		var found *core.Session
		var err error

		found, err = client.FindSession(base, apiKey, agentId, ref)

		return tuiSessionSwitchMsg{found: found, err: err}
	}
}

func switchGatewayCmd(target client.Gateway) tea.Cmd {
	return func() tea.Msg {
		var base string
		var agent *core.Agent
		var session *core.Session
		var conn *websocket.Conn

		var err error

		base, err = client.ApiBase(target.Url)
		if err != nil {
			return tuiGatewaySwitchMsg{targetName: target.Name, err: err}
		}

		agent, err = client.Agent(base, target.ApiKey, "")
		if err != nil {
			return tuiGatewaySwitchMsg{targetName: target.Name, err: err}
		}

		session, err = client.Session(base, target.ApiKey, "", agent.Id)
		if err != nil {
			return tuiGatewaySwitchMsg{targetName: target.Name, err: err}
		}

		conn, err = client.Dial(target.Url, target.ApiKey)
		if err != nil {
			return tuiGatewaySwitchMsg{targetName: target.Name, err: err}
		}

		err = conn.WriteJSON(client.Frame{Type: "attach", SessionId: session.Id})
		if err != nil {
			conn.Close()

			return tuiGatewaySwitchMsg{targetName: target.Name, err: err}
		}

		return tuiGatewaySwitchMsg{
			targetName: target.Name,
			conn:       conn,
			base:       base,
			apiKey:     target.ApiKey,
			agentName:  agent.Name,
			agentId:    agent.Id,
			sessionId:  session.Id,
		}
	}
}

func pumpTuiReplies(p *tea.Program, frames <-chan client.Reply, conn *websocket.Conn, answers chan string) {
	var reply client.Reply
	var ok bool
	var content string
	var decision string

	for {
		reply, ok = <-frames
		if !ok {
			p.Send(tuiDoneMsg{errText: client.ErrGone.Error()})

			return
		}

		switch reply.Type {
		case "chunk":
			content = ""
			if reply.Chunk != nil && len(reply.Chunk.Choices) > 0 {
				content = reply.Chunk.Choices[0].Delta.Content
			}

			p.Send(tuiChunkMsg{reasoning: reply.Reasoning, content: content})
		case "tool":
			p.Send(tuiToolMsg{name: reply.Name, status: reply.Status, message: reply.Message})
		case "message":
			p.Send(tuiMessageMsg{name: reply.Name, message: reply.Message})
		case "approval_request":
			p.Send(tuiPromptMsg{kind: "approval", sessionId: reply.SessionId, cwd: reply.Cwd, name: reply.Name, arguments: reply.Arguments})

			decision = <-answers

			conn.WriteJSON(client.Frame{Type: "approval", SessionId: reply.SessionId, Decision: decision})
		case "question_request":
			p.Send(tuiPromptMsg{kind: "question", sessionId: reply.SessionId, question: reply.Question, options: reply.Options})

			decision = <-answers

			conn.WriteJSON(client.Frame{Type: "question", SessionId: reply.SessionId, Answer: decision})
		case "error":
			p.Send(tuiDoneMsg{errText: reply.Message})
		case "done":
			p.Send(tuiDoneMsg{})
		}
	}
}

func RunTuiSession(opts client.Options) error {
	var cwd string
	var base string
	var apiKey string
	var resumed *core.Session
	var session *core.Session
	var agent *core.Agent
	var conn *websocket.Conn
	var answers chan string
	var restartPump func(*websocket.Conn)
	var p *tea.Program

	var err error

	if !client.IsTty() {
		return fmt.Errorf("stdin is not a terminal — use -p for a one-shot prompt")
	}

	cwd, err = client.ResolveCwd(opts.Cwd)
	if err != nil {
		return err
	}

	base, err = client.ApiBase(opts.Url)
	if err != nil {
		return err
	}

	apiKey = client.ResolveApiKey(opts.ApiKey, opts.Url)

	if opts.Resume && opts.Session == "" {
		resumed, err = client.LatestSession(base, apiKey, opts.Agent, cwd)
		if err != nil {
			return err
		}

		if resumed != nil {
			opts.Session = resumed.Id
		}
	}

	session, err = client.Session(base, apiKey, opts.Session, opts.Agent)
	if err != nil {
		return err
	}

	agent, err = client.Agent(base, apiKey, session.AgentId)
	if err != nil {
		return err
	}

	conn, err = client.Dial(opts.Url, apiKey)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.WriteJSON(client.Frame{Type: "attach", SessionId: session.Id})
	if err != nil {
		return err
	}

	answers = make(chan string)

	restartPump = func(next *websocket.Conn) {
		go pumpTuiReplies(p, client.Pump(next), next, answers)
	}

	p = tea.NewProgram(
		newTuiSessionModel(agent.Name, agent.Id, session.Id, cwd, base, apiKey, fetchMode(base, apiKey, cwd), opts.NoCache, conn, answers, opts.Gateways, restartPump),
		tea.WithAltScreen())

	go pumpTuiReplies(p, client.Pump(conn), conn, answers)

	_, err = p.Run()

	return err
}
