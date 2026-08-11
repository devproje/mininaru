// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
)

type chatDeltaMsg string

type chatThinkMsg string

type toolEventMsg core.ToolEvent

type chatDoneMsg struct {
	message *core.Message
	err     error
}

type toolApprovalMsg struct {
	name      string
	arguments string
	response  chan bool
}

type transcriptEntry struct {
	kind    string
	role    string
	content string
	tool    core.ToolEvent
}

type client struct {
	session *core.Session
	agent   *core.NaruAgent
	program *tea.Program

	input   textarea.Model
	spinner spinner.Model

	notice string

	pending    strings.Builder
	thinking   strings.Builder
	transcript []transcriptEntry
	sending    bool
	approval   *toolApprovalMsg
	cancel     context.CancelFunc
	err        error

	width        int
	height       int
	started      bool
	quitting     bool
	stored       bool
	scrollOffset int
}

const (
	transcriptMessage  = "message"
	transcriptThinking = "thinking"
	transcriptTool     = "tool"
	transcriptNotice   = "notice"
)

func newClient(session *core.Session, agent *core.NaruAgent, history []*core.Message) *client {
	var input textarea.Model
	var sp spinner.Model
	var c client
	var cur *core.Message
	var calls []*core.ToolCall
	var call *core.ToolCall
	var event core.ToolEvent

	var err error

	input = textarea.New()
	input.Placeholder = "Ask anything, or press ctrl+c to quit"
	input.ShowLineNumbers = false
	input.SetHeight(1)
	input.MaxHeight = 8
	input.SetPromptFunc(2, promptFunc)
	input.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "newline"))
	input.FocusedStyle.Base = lipgloss.NewStyle()
	input.BlurredStyle.Base = lipgloss.NewStyle()
	input.Focus()

	sp = spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(statusStyle))

	c = client{
		session: session,
		agent:   agent,
		input:   input,
		spinner: sp,
		notice:  updateNotice(),
		width:   80,
		height:  24,
		stored:  len(history) > 0,
	}
	for _, cur = range history {
		if cur.Reasoning != "" && config.Client.Thinking.Show {
			c.transcript = append(c.transcript, transcriptEntry{kind: transcriptThinking, content: cur.Reasoning})
		}
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: cur.Role, content: cur.Content})
		if cur.Role != "user" {
			continue
		}
		calls, err = core.ToolCallList(cur.Id)
		if err != nil {
			c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: "tool log error: " + err.Error()})
			continue
		}
		for _, call = range calls {
			event = core.ToolEvent{Phase: core.ToolEventFinished, CallId: call.CallId, Name: call.Name, Arguments: call.Arguments,
				Result: call.Result, Status: call.Status, Error: call.Error}
			c.transcript = append(c.transcript, transcriptEntry{kind: transcriptTool, tool: event})
		}
	}

	return &c
}

func runClient(session *core.Session, agent *core.NaruAgent, history []*core.Message) error {
	var c *client
	var p *tea.Program
	var release func()

	var err error

	c = newClient(session, agent, history)
	p = tea.NewProgram(c, tea.WithAltScreen())
	c.program = p

	release = util.LogHold()

	_, err = p.Run()

	release()

	if err == nil {
		fmt.Print(c.farewell())
	}

	return err
}

func (c *client) contentWidth() int {
	if c.width < 20 {
		return 20
	}

	return c.width - 4
}

func (c *client) renderMessage(role, content string) string {
	var mark string
	var text string

	if role == "user" {
		mark = userMarkStyle.Render("> ")
		text = userTextStyle.Width(c.contentWidth()).Render(content)
	} else {
		mark = naruMarkStyle.Render("⏺ ")
		text = naruTextStyle.Width(c.contentWidth()).Render(content)
	}

	return trimRight(lipgloss.JoinHorizontal(lipgloss.Top, mark, text))
}

func trimRight(block string) string {
	var lines []string
	var index int

	lines = strings.Split(block, "\n")
	for index = range lines {
		lines[index] = strings.TrimRight(lines[index], " ")
	}

	return strings.Join(lines, "\n")
}

func (c *client) banner() string {
	var body strings.Builder

	body.WriteString(bannerStyle.Render("✻ mininaru"))
	body.WriteString(metaStyle.Render(fmt.Sprintf("  %s · %s", c.agent.Name, c.agent.Model)))
	body.WriteString("\n")
	body.WriteString(metaStyle.Render("  session " + shortId(c.session.Id)))

	if c.notice != "" {
		body.WriteString("\n")
		body.WriteString(metaStyle.Render("  " + c.notice))
	}

	return body.String()
}

func (c *client) sendPrompt(ctx context.Context, content string) tea.Cmd {
	return func() tea.Msg {
		var message *core.Message

		var err error

		message, err = core.ChatWithApproval(ctx, c.session, c.agent, content, func(delta string) {
			c.program.Send(chatDeltaMsg(delta))
		}, func(delta string) {
			c.program.Send(chatThinkMsg(delta))
		}, func(event core.ToolEvent) {
			c.program.Send(toolEventMsg(event))
		}, func(approvalCtx context.Context, def modules.Def, arguments string) (bool, error) {
			var request toolApprovalMsg
			var approved bool

			request = toolApprovalMsg{name: def.Name, arguments: arguments, response: make(chan bool, 1)}
			c.program.Send(request)

			select {
			case approved = <-request.response:
				return approved, nil
			case <-approvalCtx.Done():
				return false, approvalCtx.Err()
			}
		})

		return chatDoneMsg{message: message, err: err}
	}
}

func (c *client) submit(content string) tea.Cmd {
	var ctx context.Context
	var cancel context.CancelFunc

	ctx, cancel = context.WithCancel(context.Background())

	c.cancel = cancel
	c.sending = true
	c.stored = true
	c.err = nil
	c.input.Reset()
	c.input.Blur()
	c.growInput()
	c.scrollOffset = 0
	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: "user", content: content})

	return tea.Batch(
		c.spinner.Tick,
		c.sendPrompt(ctx, content),
	)
}

func (c *client) finish(msg chatDoneMsg) tea.Cmd {
	var reply string
	var cmds []tea.Cmd

	reply = c.pending.String()

	if c.thinkingVisible() {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptThinking, content: c.thinking.String()})
	}

	c.sending = false
	c.approval = nil
	c.cancel = nil
	c.pending.Reset()
	c.thinking.Reset()
	c.input.Focus()

	if msg.err != nil {
		if !errors.Is(msg.err, context.Canceled) {
			c.err = msg.err

			return tea.Batch(append(cmds, textarea.Blink)...)
		}

		if reply != "" {
			c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: "assistant", content: reply})
		}

		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: "interrupted"})
		cmds = append(cmds, textarea.Blink)

		return tea.Batch(cmds...)
	}

	if msg.message != nil {
		reply = msg.message.Content
	}

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: "assistant", content: reply})
	cmds = append(cmds, textarea.Blink)

	return tea.Batch(cmds...)
}

func thinkingNext(level string) string {
	var levels []string
	var index int

	levels = config.ThinkingLevels()

	for index = range levels {
		if levels[index] != level {
			continue
		}

		return levels[(index+1)%len(levels)]
	}

	return config.ThinkingOff
}

func (c *client) saveThinking() tea.Cmd {
	var state string

	var err error

	err = config.ClientSave()
	if err != nil {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: err.Error()})
		return nil
	}

	state = "hidden"
	if config.Client.Thinking.Show {
		state = "shown"
	}
	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice,
		content: "thinking " + config.Client.Thinking.Level + ", " + state})
	return nil
}

func (c *client) thinkingCommand(arg string) tea.Cmd {
	if arg == "" {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice,
			content: "thinking " + config.Client.Thinking.Level})
		return nil
	}

	if arg == "show" || arg == "hide" {
		config.Client.Thinking.Show = arg == "show"

		return c.saveThinking()
	}

	if !config.ThinkingValid(arg) {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice,
			content: "unknown level " + arg + ", expected one of " + strings.Join(config.ThinkingLevels(), ", ") + ", show or hide"})
		return nil
	}

	config.Client.Thinking.Level = arg

	return c.saveThinking()
}

func (c *client) helpCommand() tea.Cmd {
	var body strings.Builder

	body.WriteString(hintStyle.Render("  /thinking                 show the current thinking setting"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /thinking <" + strings.Join(config.ThinkingLevels(), "|") + ">  set how hard the model thinks"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /thinking <show|hide>     toggle the thinking stream"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /help                     this list"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  ctrl+t                    cycle thinking level"))

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: body.String()})
	return nil
}

func (c *client) runCommand(input string) tea.Cmd {
	var fields []string
	var name string
	var arg string

	fields = strings.Fields(input)
	name = strings.ToLower(strings.TrimPrefix(fields[0], "/"))

	if len(fields) > 1 {
		arg = strings.ToLower(fields[1])
	}

	c.input.Reset()
	c.growInput()

	if name == "thinking" {
		return c.thinkingCommand(arg)
	}

	if name == "help" {
		return c.helpCommand()
	}

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: "unknown command /" + name + ", try /help"})
	return nil
}

func (c *client) Init() tea.Cmd {
	return textarea.Blink
}

func (c *client) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var windowMsg tea.WindowSizeMsg
	var keyMsg tea.KeyMsg
	var content string
	var deltaMsg chatDeltaMsg
	var thinkMsg chatThinkMsg
	var eventMsg toolEventMsg
	var approvalMsg toolApprovalMsg
	var doneMsg chatDoneMsg
	var tickMsg spinner.TickMsg
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg.(type) {
	case tea.WindowSizeMsg:
		windowMsg = msg.(tea.WindowSizeMsg)
		c.width = windowMsg.Width
		c.height = windowMsg.Height
		c.input.SetWidth(windowMsg.Width - 4)

		if c.started {
			return c, nil
		}

		c.started = true

		return c, nil

	case tea.KeyMsg:
		keyMsg = msg.(tea.KeyMsg)
		if c.approval != nil {
			if keyMsg.Type == tea.KeyCtrlC {
				if c.cancel != nil {
					c.cancel()
				}
				c.quitting = true
				return c, tea.Quit
			}
			if keyMsg.String() == "y" {
				c.approval.response <- true
				c.approval = nil
				return c, nil
			}
			if keyMsg.String() == "n" || keyMsg.Type == tea.KeyEsc {
				c.approval.response <- false
				c.approval = nil
				return c, nil
			}

			return c, nil
		}
		if keyMsg.Type == tea.KeyPgUp {
			c.scrollOffset += max(1, (c.height-8)/2)
			return c, nil
		}
		if keyMsg.Type == tea.KeyPgDown {
			c.scrollOffset -= max(1, (c.height-8)/2)
			if c.scrollOffset < 0 {
				c.scrollOffset = 0
			}
			return c, nil
		}
		if keyMsg.Type == tea.KeyEnd {
			c.scrollOffset = 0
			return c, nil
		}
		switch keyMsg.Type {
		case tea.KeyCtrlC:
			if c.cancel != nil {
				c.cancel()
			}

			c.quitting = true

			return c, tea.Quit

		case tea.KeyEsc:
			if !c.sending {
				return c, nil
			}

			c.cancel()

			return c, nil

		case tea.KeyEnter:
			if c.sending {
				return c, nil
			}

			content = strings.TrimSpace(c.input.Value())
			if content == "" {
				return c, nil
			}

			if strings.HasPrefix(content, "/") {
				return c, c.runCommand(content)
			}

			return c, c.submit(content)

		case tea.KeyCtrlT:
			if c.sending {
				return c, nil
			}

			config.Client.Thinking.Level = thinkingNext(config.Client.Thinking.Level)

			return c, c.saveThinking()
		}

	case chatDeltaMsg:
		deltaMsg = msg.(chatDeltaMsg)
		c.pending.WriteString(string(deltaMsg))

		return c, nil

	case chatThinkMsg:
		thinkMsg = msg.(chatThinkMsg)
		c.thinking.WriteString(string(thinkMsg))

		return c, nil

	case toolEventMsg:
		eventMsg = msg.(toolEventMsg)
		c.scrollOffset = 0
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptTool, tool: core.ToolEvent(eventMsg)})

		return c, nil

	case toolApprovalMsg:
		approvalMsg = msg.(toolApprovalMsg)
		c.approval = &approvalMsg

		return c, nil

	case chatDoneMsg:
		doneMsg = msg.(chatDoneMsg)
		return c, c.finish(doneMsg)

	case spinner.TickMsg:
		tickMsg = msg.(spinner.TickMsg)
		if !c.sending {
			return c, nil
		}

		c.spinner, cmd = c.spinner.Update(tickMsg)

		return c, cmd
	}

	if c.sending {
		return c, nil
	}

	c.input, cmd = c.input.Update(msg)
	cmds = append(cmds, cmd)

	c.growInput()

	return c, tea.Batch(cmds...)
}

func (c *client) growInput() {
	var lines int

	lines = c.input.LineCount()
	if lines < 1 {
		lines = 1
	}

	if lines > c.input.MaxHeight {
		lines = c.input.MaxHeight
	}

	if lines == c.input.Height() {
		return
	}

	c.input.SetHeight(lines)
}

func (c *client) renderThinking(content string) string {
	var mark string
	var text string

	mark = thinkMarkStyle.Render("✻ ")
	text = thinkTextStyle.Width(c.contentWidth()).Render(content)

	return trimRight(lipgloss.JoinHorizontal(lipgloss.Top, mark, text))
}

func compactToolLog(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		return text[:240] + "…"
	}

	return text
}

func (c *client) renderToolEvent(event core.ToolEvent) string {
	var mark string
	var detail string

	if event.Phase == core.ToolEventStarted {
		mark = "⚙"
		detail = compactToolLog(event.Arguments)
	} else if event.Status == core.MessageCompleted {
		mark = "✓"
		detail = compactToolLog(event.Result)
	} else {
		mark = "✗"
		detail = compactToolLog(event.Error)
	}
	if detail != "" {
		detail = "  " + detail
	}

	return toolMarkStyle.Render("  "+mark+" "+event.Name) + hintStyle.Render(detail)
}

func (c *client) renderToolCall(call *core.ToolCall) string {
	var event core.ToolEvent

	event = core.ToolEvent{
		Phase: core.ToolEventFinished, CallId: call.CallId, Name: call.Name, Arguments: call.Arguments,
		Result: call.Result, Status: call.Status, Error: call.Error,
	}

	return c.renderToolEvent(event)
}

func (c *client) thinkingVisible() bool {
	return config.Client.Thinking.Show && c.thinking.Len() > 0
}

func (c *client) streamView() string {
	var blocks []string

	if c.thinkingVisible() {
		blocks = append(blocks, c.renderThinking(c.thinking.String()))
	}

	if c.pending.Len() > 0 {
		blocks = append(blocks, c.renderMessage("assistant", c.pending.String()))
	}

	if len(blocks) == 0 {
		return ""
	}

	return strings.Join(blocks, "\n\n")
}

func (c *client) renderTranscriptEntry(entry transcriptEntry) string {
	if entry.kind == transcriptMessage {
		return c.renderMessage(entry.role, entry.content)
	}
	if entry.kind == transcriptThinking {
		return c.renderThinking(entry.content)
	}
	if entry.kind == transcriptTool {
		return c.renderToolEvent(entry.tool)
	}

	return hintStyle.Render("  " + entry.content)
}

func (c *client) transcriptView(height int) string {
	var entry transcriptEntry
	var blocks []string
	var stream string
	var lines []string
	var maxOffset int
	var end int
	var start int
	var body string

	for _, entry = range c.transcript {
		blocks = append(blocks, c.renderTranscriptEntry(entry))
	}
	stream = c.streamView()
	if stream != "" {
		blocks = append(blocks, stream)
	}
	if len(blocks) > 0 {
		lines = strings.Split(strings.Join(blocks, "\n\n"), "\n")
	}

	if height < 1 {
		height = 1
	}
	maxOffset = max(0, len(lines)-height)
	if c.scrollOffset > maxOffset {
		c.scrollOffset = maxOffset
	}
	end = len(lines) - c.scrollOffset
	start = max(0, end-height)
	if end > start {
		body = strings.Join(lines[start:end], "\n")
	}

	return lipgloss.NewStyle().Height(height).MaxHeight(height).Render(body)
}

func (c *client) hintText() string {
	var level string
	var options []string
	var cur string

	level = config.Client.Thinking.Level

	options = []string{
		"enter send · ctrl+j newline · ctrl+t thinking:" + level + " · /help · ctrl+c quit",
		"ctrl+j newline · ctrl+t thinking:" + level + " · /help · ctrl+c quit",
		"ctrl+t thinking:" + level + " · /help · ctrl+c quit",
		"thinking:" + level + " · /help",
	}

	for _, cur = range options {
		if lipgloss.Width(cur)+2 > c.width {
			continue
		}

		return cur
	}

	return "thinking:" + level
}

func (c *client) statusView() string {
	var arguments string

	if c.approval != nil {
		arguments = c.approval.arguments
		if len(arguments) > 120 {
			arguments = arguments[:120] + "…"
		}
		return statusStyle.Render("  approve "+c.approval.name+"? [y/N]") + hintStyle.Render("  "+arguments)
	}

	if c.sending {
		return statusStyle.Render("  "+c.spinner.View()) + hintStyle.Render("  thinking… (esc to interrupt)")
	}

	if c.err != nil {
		return errStyle.Render("  " + c.err.Error())
	}

	return hintStyle.Render("  " + c.hintText())
}

func (c *client) farewell() string {
	var body strings.Builder

	if !c.stored {
		return metaStyle.Render("  no messages yet, nothing to resume") + "\n"
	}

	body.WriteString(metaStyle.Render("  session saved, resume it with"))
	body.WriteString("\n")
	body.WriteString(naruMarkStyle.Render("    mininaru --session " + c.session.Id))
	body.WriteString("\n")
	body.WriteString(metaStyle.Render("  or --session on its own to pick up the latest one"))
	body.WriteString("\n")

	return body.String()
}

func (c *client) View() string {
	var box lipgloss.Style
	var input string
	var status string
	var historyHeight int
	var body strings.Builder

	if c.quitting {
		return ""
	}

	box = boxStyle
	if c.sending {
		box = boxBusyStyle
	}

	input = box.Width(c.width - 2).Render(c.input.View())
	status = c.statusView()
	historyHeight = c.height - lipgloss.Height(c.banner()) - lipgloss.Height(input) - lipgloss.Height(status) - 1

	body.WriteString(c.banner())
	body.WriteString("\n\n")
	body.WriteString(c.transcriptView(historyHeight))
	body.WriteString("\n")
	body.WriteString(input)
	body.WriteString("\n")
	body.WriteString(status)

	return body.String()
}

func promptFunc(line int) string {
	if line == 0 {
		return naruMarkStyle.Render("> ")
	}

	return "  "
}

func shortId(id string) string {
	if len(id) <= 8 {
		return id
	}

	return id[:8]
}
