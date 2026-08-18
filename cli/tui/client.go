// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
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

type compactDoneMsg struct {
	compacted bool
	err       error
}

type approvalDecision int

type toolApprovalMsg struct {
	name      string
	arguments string
	response  chan approvalDecision
}

type transcriptEntry struct {
	kind    string
	role    string
	content string
	tool    core.ToolEvent
}

type toolDisplayArgs struct {
	Command   string `json:"command"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type slashCommand struct {
	name        string
	description string
}

type client struct {
	session *core.Session
	agent   *core.NaruAgent
	program *tea.Program

	input   textarea.Model
	spinner spinner.Model
	view    viewport.Model
	hilView viewport.Model

	notice string

	pending    strings.Builder
	thinking   strings.Builder
	transcript []transcriptEntry
	markdown   map[string]string
	sending    bool
	compacting bool
	approval   *toolApprovalMsg
	approvalAt int
	slashOpen  bool
	slashAt    int
	allowed    map[string]bool
	allowMu    sync.Mutex
	cancel     context.CancelFunc
	err        error

	width         int
	height        int
	started       bool
	quitting      bool
	stored        bool
	newOutput     bool
	contextTokens int64
	contextWindow int64
	contextKnown  bool
}

const (
	approvalDeny approvalDecision = iota
	approvalOnce
	approvalSession
)

var slashCommands = []slashCommand{
	{name: "/thinking", description: "show or change thinking"},
	{name: "/usage", description: "show session token usage"},
	{name: "/compact", description: "compact conversation context"},
	{name: "/help", description: "show command help"},
	{name: "/exit", description: "leave the chat"},
	{name: "/quit", description: "leave the chat"},
}

const (
	transcriptMessage  = "message"
	transcriptThinking = "thinking"
	transcriptTool     = "tool"
	transcriptNotice   = "notice"
)

func approvalChoices(name string) []string {
	return []string{
		"Allow once",
		"Allow " + name + " for the rest of this session",
		"Deny",
	}
}

func approvalAt(index int) approvalDecision {
	if index == 1 {
		return approvalSession
	}

	if index == 0 {
		return approvalOnce
	}

	return approvalDeny
}

func (c *client) sessionAllowed(name string) bool {
	var allowed bool

	c.allowMu.Lock()
	allowed = c.allowed[name]
	c.allowMu.Unlock()

	return allowed
}

func (c *client) recordApproval(name string, decision approvalDecision) bool {
	if decision == approvalSession {
		c.allowMu.Lock()
		c.allowed[name] = true
		c.allowMu.Unlock()
	}

	return decision != approvalDeny
}

func newClient(session *core.Session, agent *core.NaruAgent, history []*core.Message) *client {
	var input textarea.Model
	var sp spinner.Model
	var view viewport.Model
	var hilView viewport.Model
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
	view = viewport.New(80, 1)
	view.MouseWheelEnabled = true
	hilView = viewport.New(80, 1)
	hilView.MouseWheelEnabled = true

	c = client{
		session:  session,
		agent:    agent,
		input:    input,
		spinner:  sp,
		view:     view,
		hilView:  hilView,
		allowed:  make(map[string]bool),
		markdown: make(map[string]string),
		width:    80,
		height:   24,
		stored:   len(history) > 0,
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
	c.refreshContextUsage()
	c.refreshViewport(true)

	return &c
}

func Run(session *core.Session, agent *core.NaruAgent, history []*core.Message, notice string) error {
	var c *client
	var p *tea.Program
	var release func()

	var err error

	agent.ModelContextWindow(context.Background())
	c = newClient(session, agent, history)
	c.notice = notice
	// Do not enable terminal mouse reporting here. It steals ordinary drag
	// selection from the terminal, which prevents users from copying chat text
	// with the mouse. Keyboard scrolling remains available via PageUp/PageDown.
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
	if c.width < 12 {
		return 4
	}

	return c.width - 8
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

func (c *client) renderMessage(role, content string) string {
	var mark string
	var text string

	if role == "user" {
		mark = userMarkStyle.Render("> ")
		text = userTextStyle.Width(c.contentWidth()).Render(content)
	} else {
		mark = naruMarkStyle.Render("⏺ ")
		text = c.renderMarkdown(content)
	}

	return trimRight(lipgloss.JoinHorizontal(lipgloss.Top, mark, text))
}

func (c *client) renderMarkdown(content string) string {
	var renderer *glamour.TermRenderer
	var style ansi.StyleConfig
	var rendered string
	var found bool
	var margin uint

	var err error

	rendered, found = c.markdown[content]
	if found {
		return rendered
	}

	style = styles.DarkStyleConfig
	margin = 0
	style.Document.Margin = &margin
	style.Document.BlockPrefix = ""
	style.Document.BlockSuffix = ""
	renderer, err = glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(c.contentWidth()),
	)
	if err != nil {
		return naruTextStyle.Width(c.contentWidth()).Render(content)
	}

	rendered, err = renderer.Render(content)
	if err != nil {
		return naruTextStyle.Width(c.contentWidth()).Render(content)
	}

	rendered = strings.Trim(rendered, "\n")
	c.markdown[content] = rendered

	return rendered
}

func (c *client) renderPending(content string) string {
	var mark string
	var text string

	mark = naruMarkStyle.Render("⏺ ")
	text = naruTextStyle.Width(c.contentWidth()).Render(content)

	return trimRight(lipgloss.JoinHorizontal(lipgloss.Top, mark, text))
}

func shortId(id string) string {
	if len(id) <= 8 {
		return id
	}

	return id[:8]
}

func (c *client) banner() string {
	var body strings.Builder
	var banner string

	body.WriteString(bannerStyle.Render("✻ mininaru"))
	if c.width < 56 {
		body.WriteString(metaStyle.Render(" · " + c.agent.Name + " · " + shortId(c.session.Id)))
		banner = body.String()
		return lipgloss.NewStyle().MaxWidth(max(1, c.width-4)).Render(banner)
	}

	body.WriteString(metaStyle.Render(fmt.Sprintf("  %s · %s", c.agent.Name, c.agent.Model)))
	body.WriteString("\n")
	body.WriteString(metaStyle.Render("  session " + shortId(c.session.Id)))

	if c.notice != "" {
		body.WriteString("\n")
		body.WriteString(metaStyle.Render("  " + c.notice))
	}

	return lipgloss.NewStyle().MaxWidth(max(1, c.width)).Render(body.String())
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
			var decision approvalDecision

			if c.sessionAllowed(def.Name) {
				return true, nil
			}

			request = toolApprovalMsg{name: def.Name, arguments: arguments,
				response: make(chan approvalDecision, 1)}
			c.program.Send(request)

			select {
			case decision = <-request.response:
				return c.recordApproval(def.Name, decision), nil
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
	c.compacting = false
	c.stored = true
	c.err = nil
	c.input.Reset()
	c.input.Blur()
	c.growInput()
	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: "user", content: content})
	c.refreshViewport(true)

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
	c.compacting = false
	c.approval = nil
	c.cancel = nil
	c.pending.Reset()
	c.thinking.Reset()
	c.input.Focus()
	c.refreshContextUsage()

	if msg.err != nil {
		if !errors.Is(msg.err, context.Canceled) {
			c.err = msg.err
			c.refreshViewport(false)

			return tea.Batch(append(cmds, textarea.Blink)...)
		}

		if reply != "" {
			c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: "assistant", content: reply})
		}

		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: "interrupted"})
		c.refreshViewport(false)
		cmds = append(cmds, textarea.Blink)

		return tea.Batch(cmds...)
	}

	if msg.message != nil {
		reply = msg.message.Content
	}

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: "assistant", content: reply})
	c.refreshViewport(false)
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
		c.refreshViewport(false)
		return nil
	}

	state = "hidden"
	if config.Client.Thinking.Show {
		state = "shown"
	}
	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice,
		content: "thinking " + config.Client.Thinking.Level + ", " + state})
	c.refreshViewport(false)
	return nil
}

func (c *client) thinkingCommand(arg string) tea.Cmd {
	if arg == "" {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice,
			content: "thinking " + config.Client.Thinking.Level})
		c.refreshViewport(false)
		return nil
	}

	if arg == "show" || arg == "hide" {
		config.Client.Thinking.Show = arg == "show"

		return c.saveThinking()
	}

	if !config.ThinkingValid(arg) {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice,
			content: "unknown level " + arg + ", expected one of " + strings.Join(config.ThinkingLevels(), ", ") + ", show or hide"})
		c.refreshViewport(false)
		return nil
	}

	config.Client.Thinking.Level = arg

	return c.saveThinking()
}

func (c *client) compactRun(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		var compacted bool

		var err error

		compacted, err = core.CompactNow(ctx, c.agent, c.session)

		return compactDoneMsg{compacted: compacted, err: err}
	}
}

func (c *client) compactCommand() tea.Cmd {
	var ctx context.Context
	var cancel context.CancelFunc

	ctx, cancel = context.WithCancel(context.Background())

	c.cancel = cancel
	c.sending = true
	c.compacting = true
	c.err = nil
	c.input.Blur()
	c.view.GotoBottom()
	c.newOutput = false

	return tea.Batch(c.spinner.Tick, c.compactRun(ctx))
}

func (c *client) finishCompact(msg compactDoneMsg) tea.Cmd {
	var notice string

	c.sending = false
	c.compacting = false
	c.cancel = nil
	c.input.Focus()
	c.refreshContextUsage()

	notice = "compacted the conversation context; usage refreshes after the next response"

	if msg.err != nil {
		notice = "could not compact: " + msg.err.Error()
		if errors.Is(msg.err, context.Canceled) {
			notice = "compacting was interrupted"
		}
	} else if !msg.compacted {
		notice = "nothing to compact yet"
	}

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: notice})
	c.refreshViewport(false)

	return textarea.Blink
}

func (c *client) usageCommand() tea.Cmd {
	var totals *core.UsageTotals
	var notice string

	var err error

	totals, err = core.SessionUsage(c.session.Id)
	if err != nil {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice,
			content: "could not read token usage: " + err.Error()})
		c.refreshViewport(false)

		return nil
	}

	notice = fmt.Sprintf("%d tokens this session (%d prompt, %d completion, %d cache read, %d cache write)",
		totals.TotalTokens, totals.PromptTokens, totals.CompletionTokens, totals.CachedTokens, totals.CacheWriteTokens)

	if totals.TotalTokens == 0 {
		notice = "no token usage recorded for this session yet"
	}

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: notice})
	c.refreshViewport(false)

	return nil
}

func (c *client) exitCommand() tea.Cmd {
	if c.cancel != nil {
		c.cancel()
	}

	c.quitting = true

	return tea.Quit
}

func (c *client) helpCommand() tea.Cmd {
	var body strings.Builder

	body.WriteString(hintStyle.Render("  /thinking                 show the current thinking setting"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /thinking <" + strings.Join(config.ThinkingLevels(), "|") + ">  set how hard the model thinks"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /thinking <show|hide>     toggle the thinking stream"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /usage                    tokens this session has spent"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /compact                  fold this conversation into a summary now"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /exit, /quit              leave the chat"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  /help                     this list"))
	body.WriteString("\n")
	body.WriteString(hintStyle.Render("  ctrl+t                    cycle thinking level"))

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: body.String()})
	c.refreshViewport(false)
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
	c.slashOpen = false
	c.slashAt = 0
	c.growInput()

	if name == "thinking" {
		return c.thinkingCommand(arg)
	}

	if name == "compact" {
		return c.compactCommand()
	}

	if name == "usage" {
		return c.usageCommand()
	}

	if name == "exit" || name == "quit" {
		return c.exitCommand()
	}

	if name == "help" {
		return c.helpCommand()
	}

	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: "unknown command /" + name + ", try /help"})
	c.refreshViewport(false)
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
	var compactMsg compactDoneMsg
	var tickMsg spinner.TickMsg
	var index int
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg.(type) {
	case tea.WindowSizeMsg:
		windowMsg = msg.(tea.WindowSizeMsg)
		if c.width != windowMsg.Width {
			c.markdown = make(map[string]string)
		}
		c.width = windowMsg.Width
		c.height = windowMsg.Height
		c.input.SetWidth(max(1, windowMsg.Width-8))
		c.resizeViewport()

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
			if keyMsg.Type == tea.KeyUp || keyMsg.String() == "k" {
				if c.approvalAt > 0 {
					c.approvalAt--
				}
				return c, nil
			}
			if keyMsg.Type == tea.KeyDown || keyMsg.String() == "j" {
				if c.approvalAt < len(approvalChoices(c.approval.name))-1 {
					c.approvalAt++
				}
				return c, nil
			}
			if keyMsg.Type == tea.KeyPgUp {
				c.hilView.PageUp()
				return c, nil
			}
			if keyMsg.Type == tea.KeyPgDown {
				c.hilView.PageDown()
				return c, nil
			}
			if keyMsg.Type == tea.KeyHome {
				c.hilView.GotoTop()
				return c, nil
			}
			if keyMsg.Type == tea.KeyEnd {
				c.hilView.GotoBottom()
				return c, nil
			}
			if keyMsg.Type == tea.KeyEnter {
				c.approval.response <- approvalAt(c.approvalAt)
				c.approval = nil

				return c, nil
			}
			if keyMsg.Type == tea.KeyEsc {
				c.approval.response <- approvalDeny
				c.approval = nil

				return c, nil
			}

			return c, nil
		}
		if c.slashOpen {
			if keyMsg.Type == tea.KeyEsc {
				c.slashOpen = false
				return c, nil
			}
			if keyMsg.Type == tea.KeyUp {
				if c.slashAt > 0 {
					c.slashAt--
				}
				return c, nil
			}
			if keyMsg.Type == tea.KeyDown {
				if c.slashAt < len(c.filteredSlashCommands())-1 {
					c.slashAt++
				}
				return c, nil
			}
			if keyMsg.Type == tea.KeyTab || keyMsg.Type == tea.KeyEnter {
				return c, c.selectSlashCommand(keyMsg.Type == tea.KeyEnter)
			}
		}
		if keyMsg.Type == tea.KeyPgUp {
			c.view.PageUp()
			return c, nil
		}
		if keyMsg.Type == tea.KeyPgDown {
			c.view.PageDown()
			if c.view.AtBottom() {
				c.newOutput = false
			}
			return c, nil
		}
		if keyMsg.Type == tea.KeyEnd {
			c.view.GotoBottom()
			c.newOutput = false
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
		c.refreshViewport(false)

		return c, nil

	case chatThinkMsg:
		thinkMsg = msg.(chatThinkMsg)
		c.thinking.WriteString(string(thinkMsg))
		c.refreshViewport(false)

		return c, nil

	case toolEventMsg:
		eventMsg = msg.(toolEventMsg)
		if eventMsg.Phase == core.ToolEventFinished {
			for index = len(c.transcript) - 1; index >= 0; index-- {
				if c.transcript[index].kind != transcriptTool || c.transcript[index].tool.CallId != eventMsg.CallId {
					continue
				}
				c.transcript[index].tool = core.ToolEvent(eventMsg)
				c.refreshViewport(false)
				return c, nil
			}
		}
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptTool, tool: core.ToolEvent(eventMsg)})
		c.refreshViewport(false)

		return c, nil

	case toolApprovalMsg:
		approvalMsg = msg.(toolApprovalMsg)
		c.approval = &approvalMsg
		c.approvalAt = 0
		c.resizeViewport()
		c.hilView.GotoTop()

		return c, nil

	case chatDoneMsg:
		doneMsg = msg.(chatDoneMsg)
		return c, c.finish(doneMsg)

	case compactDoneMsg:
		compactMsg = msg.(compactDoneMsg)
		return c, c.finishCompact(compactMsg)

	case spinner.TickMsg:
		tickMsg = msg.(spinner.TickMsg)
		if !c.sending {
			return c, nil
		}

		c.spinner, cmd = c.spinner.Update(tickMsg)

		return c, cmd

	case tea.MouseMsg:
		if c.approval != nil {
			c.hilView, cmd = c.hilView.Update(msg)
			return c, cmd
		}

		c.view, cmd = c.view.Update(msg)
		if c.view.AtBottom() {
			c.newOutput = false
		}
		return c, cmd
	}

	if c.sending {
		return c, nil
	}

	c.input, cmd = c.input.Update(msg)
	cmds = append(cmds, cmd)
	c.updateSlashMenu()

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

func (c *client) filteredSlashCommands() []slashCommand {
	var prefix string
	var command slashCommand
	var commands []slashCommand
	var level string

	prefix = strings.ToLower(c.input.Value())
	if strings.HasPrefix(prefix, "/thinking ") {
		for _, level = range config.ThinkingLevels() {
			command = slashCommand{name: "/thinking " + level, description: "set thinking level"}
			if strings.HasPrefix(command.name, prefix) {
				commands = append(commands, command)
			}
		}
		for _, level = range []string{"show", "hide"} {
			command = slashCommand{name: "/thinking " + level, description: level + " thinking output"}
			if strings.HasPrefix(command.name, prefix) {
				commands = append(commands, command)
			}
		}
		return commands
	}
	for _, command = range slashCommands {
		if strings.HasPrefix(command.name, prefix) {
			commands = append(commands, command)
		}
	}

	return commands
}

func (c *client) updateSlashMenu() {
	var value string
	var commands []slashCommand

	value = c.input.Value()
	c.slashOpen = strings.HasPrefix(value, "/") && !strings.Contains(value, "\n")
	commands = c.filteredSlashCommands()
	if len(commands) == 0 {
		c.slashOpen = false
		c.slashAt = 0
		return
	}
	if c.slashAt >= len(commands) {
		c.slashAt = len(commands) - 1
	}
}

func (c *client) selectSlashCommand(run bool) tea.Cmd {
	var commands []slashCommand
	var selected string

	commands = c.filteredSlashCommands()
	if len(commands) == 0 {
		c.slashOpen = false
		return nil
	}
	selected = commands[c.slashAt].name
	c.input.SetValue(selected)
	c.input.CursorEnd()
	c.slashOpen = false
	c.slashAt = 0
	if run {
		return c.runCommand(selected)
	}

	return nil
}

func (c *client) renderThinking(content string) string {
	var renderer *glamour.TermRenderer
	var style ansi.StyleConfig
	var quoted string
	var text string
	var indent uint
	var margin uint

	var err error

	style = styles.DarkStyleConfig
	indent = 1
	margin = 0
	style.Document.Margin = &margin
	style.Document.BlockPrefix = ""
	style.Document.BlockSuffix = ""
	style.BlockQuote.Indent = &indent
	quoted = "> " + strings.ReplaceAll(content, "\n", "  \n> ")
	renderer, err = glamour.NewTermRenderer(glamour.WithStyles(style), glamour.WithWordWrap(c.contentWidth()))
	if err != nil {
		return thinkTextStyle.Width(c.contentWidth()).Render(content)
	}
	text, err = renderer.Render(quoted)
	if err != nil {
		return thinkTextStyle.Width(c.contentWidth()).Render(content)
	}

	return trimRight(strings.Trim(text, "\n"))
}

func compactToolLog(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		return text[:240] + "…"
	}

	return text
}

func boundToolDetail(text string) string {
	var lines []string
	var runes []rune

	text = strings.TrimRight(text, "\n")
	lines = strings.Split(text, "\n")
	if len(lines) > 24 {
		lines = append(lines[:24], "… output truncated")
	}
	text = strings.Join(lines, "\n")
	runes = []rune(text)
	if len(runes) > 3000 {
		text = string(runes[:3000]) + "\n… output truncated"
	}

	return text
}

func toolDiff(oldText, newText string) string {
	var oldLines []string
	var newLines []string
	var blocks []string
	var line string

	oldLines = strings.Split(oldText, "\n")
	newLines = strings.Split(newText, "\n")
	for _, line = range oldLines {
		blocks = append(blocks, toolCutStyle.Render("- "+line))
	}
	for _, line = range newLines {
		blocks = append(blocks, toolAddedStyle.Render("+ "+line))
	}

	return strings.Join(blocks, "\n")
}

func toolUnifiedDiff(text string) string {
	var blocks []string
	var line string
	var lines []string

	lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	for _, line = range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			blocks = append(blocks, toolAddedStyle.Render(line))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			blocks = append(blocks, toolCutStyle.Render(line))
		} else {
			blocks = append(blocks, line)
		}
	}

	return strings.Join(blocks, "\n")
}

func toolDisplay(event core.ToolEvent) (string, string) {
	var payload toolDisplayArgs
	var title string
	var detail string
	var mark string

	var err error

	mark = "✗"
	if event.Phase == core.ToolEventStarted {
		mark = "⚙"
	} else if event.Status == core.MessageCompleted {
		mark = "✓"
	}
	title = core.ToolLabel(event.Name, event.Arguments)
	err = json.Unmarshal([]byte(event.Arguments), &payload)
	if err != nil {
		payload = toolDisplayArgs{}
	}

	if event.Name == "bash_exec" {
		title = "Bash"
		if event.Phase == core.ToolEventStarted {
			detail = "$ " + payload.Command
		} else if event.Status == core.MessageCompleted {
			detail = "$ " + payload.Command
			if event.Result != "" {
				detail += "\n\n" + event.Result
			}
		} else {
			detail = "$ " + payload.Command + "\n\n" + event.Error
		}
	} else if event.Name == "file_edit" {
		title = "Update " + payload.Path
		if event.Phase == core.ToolEventFinished && event.Status == core.MessageCompleted {
			detail = toolUnifiedDiff(event.Result)
		} else {
			detail = toolDiff(payload.OldString, payload.NewString)
			if event.Phase == core.ToolEventFinished {
				detail += "\n\n" + event.Error
			}
		}
	} else if event.Name == "file_write" {
		title = "Write " + payload.Path
		if event.Phase == core.ToolEventFinished && event.Status == core.MessageCompleted {
			detail = toolUnifiedDiff(event.Result)
		} else {
			detail = payload.Content
			if event.Phase == core.ToolEventFinished {
				detail += "\n\n" + event.Error
			}
		}
	} else if event.Phase == core.ToolEventStarted {
		detail = compactToolLog(event.Arguments)
	} else if event.Status == core.MessageCompleted {
		detail = compactToolLog(event.Result)
	} else {
		detail = compactToolLog(event.Error)
	}

	return mark + " " + title, boundToolDetail(detail)
}

func (c *client) renderToolEvent(event core.ToolEvent) string {
	var title string
	var detail string

	title, detail = toolDisplay(event)
	if detail == "" {
		return toolMarkStyle.Render("  " + title)
	}

	return toolMarkStyle.Render("  "+title) + "\n" +
		toolBodyStyle.Width(max(1, c.contentWidth()-2)).Render(detail)
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
		blocks = append(blocks, c.renderPending(c.pending.String()))
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

func (c *client) transcriptContent() string {
	var entry transcriptEntry
	var blocks []string
	var stream string

	for _, entry = range c.transcript {
		blocks = append(blocks, c.renderTranscriptEntry(entry))
	}
	stream = c.streamView()
	if stream != "" {
		blocks = append(blocks, stream)
	}

	return strings.Join(blocks, "\n\n")
}

func (c *client) refreshViewport(forceBottom bool) {
	var follow bool

	follow = forceBottom || c.view.AtBottom()
	c.view.SetContent(c.transcriptContent())
	if follow {
		c.view.GotoBottom()
		c.newOutput = false
		return
	}

	c.newOutput = true
}

func (c *client) resizeViewport() {
	var follow bool
	var box lipgloss.Style
	var input string
	var status string
	var hilHeight int
	var slashHeight int
	var height int

	follow = c.view.AtBottom()
	box = boxStyle
	if c.sending {
		box = boxBusyStyle
	}
	input = box.Width(max(1, c.width-6)).Render(c.input.View())
	status = c.statusView()
	hilHeight = c.resizeHIL()
	slashHeight = lipgloss.Height(c.slashView())
	height = c.height - lipgloss.Height(c.banner()) - lipgloss.Height(status) - hilHeight - slashHeight - 3
	if c.approval == nil {
		height -= lipgloss.Height(input) + 1
		if slashHeight > 0 {
			height--
		}
	} else {
		height--
	}

	c.view.Width = max(1, c.width-4)
	c.view.Height = max(1, height)
	c.view.SetContent(c.transcriptContent())
	if follow {
		c.view.GotoBottom()
	}
}

func (c *client) resizeHIL() int {
	var available int
	var choicesHeight int
	var headerHeight int
	var height int

	if c.approval == nil {
		return 0
	}

	available = max(4, c.height-lipgloss.Height(c.banner())-lipgloss.Height(c.statusView())-4)
	headerHeight = lipgloss.Height(c.approvalHeader())
	choicesHeight = lipgloss.Height(c.approvalChoicesView())
	height = min(8, max(1, available-headerHeight-choicesHeight-2))
	c.hilView.Width = max(1, c.width-4)
	c.hilView.Height = height
	c.hilView.SetContent(hintStyle.Width(c.contentWidth()).Render("  " + c.approval.arguments))

	return headerHeight + height + choicesHeight + 2
}

func (c *client) hintText() string {
	var level string
	var options []string
	var cur string
	var reserved int

	level = config.Client.Thinking.Level
	reserved = lipgloss.Width(c.contextUsage()) + 3

	options = []string{
		"enter send · ctrl+j newline · ctrl+t thinking:" + level + " · /help · ctrl+c quit",
		"ctrl+j newline · ctrl+t thinking:" + level + " · /help · ctrl+c quit",
		"ctrl+t thinking:" + level + " · /help · ctrl+c quit",
		"thinking:" + level + " · /help",
	}

	for _, cur = range options {
		if lipgloss.Width(cur)+reserved > c.contentWidth() {
			continue
		}

		return cur
	}

	if lipgloss.Width("thinking:"+level)+reserved <= c.contentWidth() {
		return "thinking:" + level
	}

	return ""
}

func (c *client) slashView() string {
	var body strings.Builder
	var commands []slashCommand
	var command slashCommand
	var index int
	var line string

	if !c.slashOpen {
		return ""
	}
	commands = c.filteredSlashCommands()
	for index, command = range commands {
		if index > 0 {
			body.WriteString("\n")
		}
		line = "  " + command.name + "  " + command.description
		if index == c.slashAt {
			body.WriteString(statusStyle.MaxWidth(c.contentWidth()).Render("▸ " + line))
			continue
		}
		body.WriteString(hintStyle.MaxWidth(c.contentWidth()).Render("  " + line))
	}

	return body.String()
}

func (c *client) approvalContent() string {
	return c.approvalHeader() + "\n" + c.approval.arguments + "\n" + c.approvalChoicesView()
}

func (c *client) approvalHeader() string {
	return statusStyle.MaxWidth(c.contentWidth()).Render("  approve " + c.approval.name + "?")
}

func (c *client) approvalChoicesView() string {
	var body strings.Builder
	var choices []string
	var index int
	var choice string

	choices = approvalChoices(c.approval.name)

	for index = range choices {
		if index > 0 {
			body.WriteString("\n")
		}
		choice = choices[index]

		if index == c.approvalAt {
			body.WriteString(statusStyle.MaxWidth(c.contentWidth()).Render("  ▸ " + choice))
			continue
		}

		body.WriteString(hintStyle.MaxWidth(c.contentWidth()).Render("    " + choice))
	}

	return body.String()
}

func compactCount(value int64) string {
	if value < 1000 {
		return fmt.Sprintf("%d", value)
	}

	return fmt.Sprintf("%.1fk", float64(value)/1000)
}

func (c *client) contextUsage() string {
	if !c.contextKnown {
		if c.contextWindow > 0 {
			return "ctx —/" + compactCount(c.contextWindow)
		}
		return "ctx —"
	}
	if c.contextWindow <= 0 {
		return "ctx " + compactCount(c.contextTokens)
	}

	return "ctx " + compactCount(c.contextTokens) + "/" + compactCount(c.contextWindow)
}

func (c *client) refreshContextUsage() {
	var tokens int64
	var window int64
	var known bool

	var err error

	tokens, window, known, err = core.SessionContextTokens(c.session.Id)
	if err != nil {
		return
	}
	c.contextTokens = tokens
	c.contextWindow = window
	c.contextKnown = known
}

func (c *client) statusLine(left string) string {
	var right string
	var gap int

	right = c.contextUsage()
	gap = c.contentWidth() - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}

	return left + strings.Repeat(" ", gap) + hintStyle.Render(right)
}

func (c *client) statusView() string {
	var activity string
	var status string

	if c.sending {
		activity = "thinking…"
		if c.compacting {
			activity = "compacting…"
		}
		status = c.statusLine(statusStyle.Render("  "+c.spinner.View()) + hintStyle.Render("  "+activity+" (esc to interrupt)"))
	} else if c.err != nil {
		status = c.statusLine(errStyle.MaxWidth(c.contentWidth()).Render("  " + c.err.Error()))
	} else {
		status = c.statusLine(hintStyle.Render("  " + c.hintText()))
	}

	if c.newOutput {
		status = status + "\n" + statusStyle.Render("  ↓ new output") + hintStyle.Render("  End to latest")
	}

	return status
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
	var body strings.Builder

	if c.quitting {
		return ""
	}

	box = boxStyle
	if c.sending {
		box = boxBusyStyle
	}

	input = box.Width(max(1, c.width-6)).Render(c.input.View())
	status = c.statusView()
	c.resizeViewport()

	body.WriteString("\n")
	body.WriteString(c.banner())
	body.WriteString("\n\n")
	body.WriteString(c.view.View())
	body.WriteString("\n")
	if c.approval != nil {
		body.WriteString(c.approvalHeader())
		body.WriteString("\n")
		body.WriteString(lipgloss.NewStyle().Height(c.hilView.Height).Render(c.hilView.View()))
		body.WriteString("\n")
		body.WriteString(c.approvalChoicesView())
		body.WriteString("\n")
	} else {
		if c.slashOpen {
			body.WriteString(c.slashView())
			body.WriteString("\n")
		}
		body.WriteString("\n")
		body.WriteString(input)
		body.WriteString("\n")
	}
	body.WriteString(status)
	body.WriteString("\n")

	return lipgloss.NewStyle().Height(c.height).Padding(0, 2).Render(body.String())
}

func promptFunc(line int) string {
	if line == 0 {
		return naruMarkStyle.Render("> ")
	}

	return "  "
}
