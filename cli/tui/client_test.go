// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

func tuiClient(t *testing.T) *client {
	var c *client

	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	config.Client.Thinking = config.Thinking{Level: config.ThinkingOff, Show: true}

	c = newClient(&core.Session{Id: "s", Name: "d"},
		&core.NaruAgent{Id: "g", Name: "naru", Model: "m"}, nil)
	c.Update(tea.WindowSizeMsg{Width: 72, Height: 24})

	return c
}

func typeEnter(c *client, text string) {
	c.input.SetValue(text)
	c.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestSlashThinkingSetsLevel(t *testing.T) {
	var c *client

	c = tuiClient(t)

	typeEnter(c, "/thinking high")
	if config.Client.Thinking.Level != "high" {
		t.Fatalf("level = %q, want high", config.Client.Thinking.Level)
	}

	if c.input.Value() != "" {
		t.Fatalf("input not cleared after command: %q", c.input.Value())
	}

	typeEnter(c, "/thinking hide")
	if config.Client.Thinking.Show {
		t.Fatal("/thinking hide did not hide the stream")
	}

	typeEnter(c, "/thinking show")
	if !config.Client.Thinking.Show {
		t.Fatal("/thinking show did not reveal the stream")
	}
}

func TestSlashThinkingPersists(t *testing.T) {
	var c *client

	var err error

	c = tuiClient(t)
	typeEnter(c, "/thinking max")

	config.Client.Thinking = config.Thinking{}

	err = config.ClientInit()
	if err != nil {
		t.Fatal(err)
	}

	if config.Client.Thinking.Level != "max" {
		t.Fatalf("level after reload = %q, want max (not saved to client.json)", config.Client.Thinking.Level)
	}
}

func TestCtrlTCyclesThroughEveryLevel(t *testing.T) {
	var c *client
	var i int
	var seen []string

	c = tuiClient(t)

	for i = 0; i < 5; i++ {
		c.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
		seen = append(seen, config.Client.Thinking.Level)
	}

	if strings.Join(seen, ",") != "low,medium,high,max,off" {
		t.Fatalf("ctrl+t cycle = %v, want low,medium,high,max,off", seen)
	}
}

func TestUnknownCommandDoesNotSendToModel(t *testing.T) {
	var c *client

	c = tuiClient(t)
	typeEnter(c, "/nope")

	if c.sending {
		t.Fatal("unknown slash command was sent to the model")
	}
}

func TestPlainTextIsNotTreatedAsCommand(t *testing.T) {
	var c *client

	c = tuiClient(t)
	c.input.SetValue("hello there")
	c.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !c.sending {
		t.Fatal("plain text should have been submitted to the model")
	}
}

func TestSlashOpensAndFiltersCommandMenu(t *testing.T) {
	var c *client
	var view string

	c = tuiClient(t)
	c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	view = c.View()
	if !c.slashOpen || !strings.Contains(view, "/thinking") || !strings.Contains(view, "/compact") {
		t.Fatalf("slash command menu did not open: %q", view)
	}

	c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	view = c.View()
	if !strings.Contains(view, "/usage") || strings.Contains(view, "/compact") {
		t.Fatalf("slash command menu did not filter: %q", view)
	}

	c.Update(tea.KeyMsg{Type: tea.KeyTab})
	if c.input.Value() != "/usage" || c.slashOpen {
		t.Fatalf("tab completed %q with menu open=%t", c.input.Value(), c.slashOpen)
	}

	c.input.SetValue("/thinking hid")
	c.updateSlashMenu()
	view = c.View()
	if !strings.Contains(view, "/thinking hide") || strings.Contains(view, "/thinking show") {
		t.Fatalf("thinking argument completion did not filter: %q", view)
	}
	c.Update(tea.KeyMsg{Type: tea.KeyTab})
	if c.input.Value() != "/thinking hide" {
		t.Fatalf("thinking argument completed to %q", c.input.Value())
	}
}

func TestStatusShowsContextUsageOnTheRight(t *testing.T) {
	var c *client
	var status string

	c = tuiClient(t)
	c.contextTokens = 9428
	c.contextWindow = 16384
	c.contextKnown = true

	status = c.statusView()
	if !strings.Contains(status, "ctx 9.4k/16.4k") {
		t.Fatalf("context usage is missing from status: %q", status)
	}
}

func TestToolApprovalArrowsAndEnter(t *testing.T) {
	var c *client
	var response chan approvalDecision
	var decision approvalDecision

	c = tuiClient(t)
	c.sending = true
	response = make(chan approvalDecision, 1)
	c.Update(toolApprovalMsg{name: "bash_exec", arguments: `{"command":"pwd"}`, response: response})

	if c.approval == nil || !strings.Contains(c.approvalContent(), "approve bash_exec") {
		t.Fatal("tool approval prompt was not displayed")
	}
	if c.approvalAt != 0 {
		t.Fatalf("cursor started at %d, want the first choice", c.approvalAt)
	}

	c.Update(tea.KeyMsg{Type: tea.KeyUp})
	if c.approvalAt != 0 {
		t.Fatal("cursor moved above the first choice")
	}

	c.Update(tea.KeyMsg{Type: tea.KeyDown})
	if c.approvalAt != 1 {
		t.Fatalf("cursor = %d after one down, want 1", c.approvalAt)
	}

	c.Update(tea.KeyMsg{Type: tea.KeyDown})
	c.Update(tea.KeyMsg{Type: tea.KeyDown})
	if c.approvalAt != 2 {
		t.Fatalf("cursor = %d, want it clamped to the last choice", c.approvalAt)
	}

	c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	decision = <-response
	if decision != approvalDeny || c.approval != nil {
		t.Fatalf("enter on the last choice sent %v", decision)
	}
}

func TestToolApprovalSelectsSessionAndResetsCursor(t *testing.T) {
	var c *client
	var response chan approvalDecision
	var decision approvalDecision

	c = tuiClient(t)
	c.sending = true
	response = make(chan approvalDecision, 1)
	c.Update(toolApprovalMsg{name: "file_write", arguments: `{}`, response: response})

	c.Update(tea.KeyMsg{Type: tea.KeyDown})
	c.Update(tea.KeyMsg{Type: tea.KeyEnter})

	decision = <-response
	if decision != approvalSession {
		t.Fatalf("decision = %v, want the session choice", decision)
	}

	response = make(chan approvalDecision, 1)
	c.Update(toolApprovalMsg{name: "bash_exec", arguments: `{}`, response: response})
	if c.approvalAt != 0 {
		t.Fatalf("cursor = %d on a new approval, want it reset", c.approvalAt)
	}
}

func TestToolApprovalEscDenies(t *testing.T) {
	var c *client
	var response chan approvalDecision
	var decision approvalDecision

	c = tuiClient(t)
	c.sending = true
	response = make(chan approvalDecision, 1)
	c.Update(toolApprovalMsg{name: "bash_exec", arguments: `{}`, response: response})

	c.Update(tea.KeyMsg{Type: tea.KeyEsc})
	decision = <-response
	if decision != approvalDeny || c.approval != nil {
		t.Fatalf("esc sent %v", decision)
	}
}

func TestToolApprovalMenuRendersEveryChoice(t *testing.T) {
	var c *client
	var view string

	c = tuiClient(t)
	c.sending = true
	c.Update(toolApprovalMsg{name: "bash_exec", arguments: `{}`,
		response: make(chan approvalDecision, 1)})

	view = c.approvalContent()

	if !strings.Contains(view, "Allow once") || !strings.Contains(view, "Deny") {
		t.Fatalf("menu is missing a choice: %q", view)
	}
	if !strings.Contains(view, "Allow bash_exec for the rest of this session") {
		t.Fatalf("the session choice does not name the tool: %q", view)
	}
	if !strings.Contains(view, "▸") {
		t.Fatalf("no cursor marker: %q", view)
	}
}

func TestToolApprovalHidesInputAndKeepsChoicesVisible(t *testing.T) {
	var c *client
	var before int
	var view string

	c = tuiClient(t)
	c.sending = true
	c.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	c.Update(toolApprovalMsg{name: "bash_exec", arguments: strings.Repeat("line\n", 20),
		response: make(chan approvalDecision, 1)})

	view = c.View()
	if strings.Contains(view, "Ask anything") {
		t.Fatal("input remained visible during tool approval")
	}
	if !strings.Contains(view, "Allow once") || !strings.Contains(view, "Deny") {
		t.Fatalf("approval choices are hidden by long arguments: %q", view)
	}

	before = c.hilView.YOffset
	c.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if c.hilView.YOffset <= before {
		t.Fatal("PageDown did not scroll the HIL content")
	}
	c.Update(tea.KeyMsg{Type: tea.KeyHome})
	if c.hilView.YOffset != before {
		t.Fatal("Home did not return to the start of the command")
	}

	c.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if c.hilView.YOffset <= before {
		t.Fatal("mouse wheel did not scroll the HIL command")
	}
}

func TestSessionApprovalRemembersOnlyThatTool(t *testing.T) {
	var c *client

	c = tuiClient(t)

	if c.sessionAllowed("bash_exec") {
		t.Fatal("nothing was allowed yet")
	}

	if !c.recordApproval("bash_exec", approvalSession) {
		t.Fatal("the session choice should allow the call")
	}
	if !c.sessionAllowed("bash_exec") {
		t.Fatal("the session choice was not remembered")
	}
	if c.sessionAllowed("file_write") {
		t.Fatal("allowing one tool allowed another")
	}

	if !c.recordApproval("file_write", approvalOnce) {
		t.Fatal("allow once should allow the call")
	}
	if c.sessionAllowed("file_write") {
		t.Fatal("allow once was remembered as a session grant")
	}

	if c.recordApproval("grep", approvalDeny) {
		t.Fatal("deny should not allow the call")
	}
}

func TestToolLogRendering(t *testing.T) {
	var c *client
	var line string

	c = tuiClient(t)
	line = c.renderToolEvent(core.ToolEvent{Phase: core.ToolEventStarted, Name: "file_read", Arguments: `{"path":"README.md"}`, Status: core.MessagePending})
	if !strings.Contains(line, "file_read") || !strings.Contains(line, "README.md") {
		t.Fatalf("started tool log = %q", line)
	}

	line = c.renderToolCall(&core.ToolCall{Name: "file_read", Result: "contents", Status: core.MessageCompleted})
	if !strings.Contains(line, "file_read") || !strings.Contains(line, "contents") {
		t.Fatalf("completed tool log = %q", line)
	}
}

func TestDetailedToolLogRendering(t *testing.T) {
	var c *client
	var rendered string

	c = tuiClient(t)
	rendered = c.renderToolEvent(core.ToolEvent{Phase: core.ToolEventStarted, Name: "bash_exec",
		Arguments: `{"command":"go test ./..."}`, Status: core.MessagePending})
	if !strings.Contains(rendered, "Bash") || !strings.Contains(rendered, "$ go test ./...") {
		t.Fatalf("bash command was not expanded: %q", rendered)
	}

	rendered = c.renderToolEvent(core.ToolEvent{Phase: core.ToolEventFinished, Name: "bash_exec",
		Arguments: `{"command":"go test ./..."}`, Result: "ok example/pkg", Status: core.MessageCompleted})
	if !strings.Contains(rendered, "✓ Bash") || !strings.Contains(rendered, "ok example/pkg") {
		t.Fatalf("bash result was not expanded: %q", rendered)
	}

	rendered = c.renderToolEvent(core.ToolEvent{Phase: core.ToolEventStarted, Name: "file_edit",
		Arguments: `{"path":"main.go","old_string":"old line","new_string":"new line"}`, Status: core.MessagePending})
	if !strings.Contains(rendered, "Update main.go") || !strings.Contains(rendered, "- old line") ||
		!strings.Contains(rendered, "+ new line") {
		t.Fatalf("file edit was not rendered as a diff: %q", rendered)
	}
	if !strings.Contains(rendered, toolCutStyle.Render("- old line")) ||
		!strings.Contains(rendered, toolAddedStyle.Render("+ new line")) {
		t.Fatalf("file edit diff was not colorized: %q", rendered)
	}

	rendered = c.renderToolEvent(core.ToolEvent{Phase: core.ToolEventFinished, Name: "file_edit",
		Arguments: `{"path":"main.go","old_string":"old line","new_string":"new line"}`,
		Result:    "--- a/main.go\n+++ b/main.go\n@@ -1,1 +1,1 @@\n-old line\n+new line\n", Status: core.MessageCompleted})
	if !strings.Contains(rendered, toolCutStyle.Render("-old line")) ||
		!strings.Contains(rendered, toolAddedStyle.Render("+new line")) {
		t.Fatalf("completed file diff was not colorized: %q", rendered)
	}

	rendered = c.renderToolEvent(core.ToolEvent{Phase: core.ToolEventStarted, Name: "file_write",
		Arguments: `{"path":"new.go","content":"package main"}`, Status: core.MessagePending})
	if !strings.Contains(rendered, "Write new.go") || !strings.Contains(rendered, "package main") {
		t.Fatalf("file write content was not expanded: %q", rendered)
	}
}

func TestCompactStatusNamesCompaction(t *testing.T) {
	var c *client
	var status string

	c = tuiClient(t)
	c.sending = true
	c.compacting = true
	status = c.statusView()
	if !strings.Contains(status, "compacting…") || strings.Contains(status, "thinking…") {
		t.Fatalf("compact status is ambiguous: %q", status)
	}
}

func TestFinishedToolEventReplacesItsRunningBlock(t *testing.T) {
	var c *client

	c = tuiClient(t)
	c.Update(toolEventMsg(core.ToolEvent{Phase: core.ToolEventStarted, CallId: "call-1", Name: "bash_exec",
		Arguments: `{"command":"pwd"}`, Status: core.MessagePending}))
	c.Update(toolEventMsg(core.ToolEvent{Phase: core.ToolEventFinished, CallId: "call-1", Name: "bash_exec",
		Arguments: `{"command":"pwd"}`, Result: "/tmp", Status: core.MessageCompleted}))

	if len(c.transcript) != 1 {
		t.Fatalf("tool call produced %d transcript blocks, want one", len(c.transcript))
	}
	if c.transcript[0].tool.Phase != core.ToolEventFinished {
		t.Fatal("running tool block was not replaced by its result")
	}
}

func TestAssistantMessagesRenderMarkdown(t *testing.T) {
	var c *client
	var rendered string

	c = tuiClient(t)
	rendered = c.renderMessage("assistant", "# Heading\n\n**bold** and `code`")

	if strings.Contains(rendered, "# Heading") || strings.Contains(rendered, "**bold**") {
		t.Fatalf("markdown syntax was not rendered: %q", rendered)
	}
	if !strings.Contains(rendered, "Heading") || !strings.Contains(rendered, "bold") || !strings.Contains(rendered, "code") {
		t.Fatalf("markdown content was lost: %q", rendered)
	}
}

func TestThinkingRendersAsAQuote(t *testing.T) {
	var c *client
	var rendered string

	c = tuiClient(t)
	rendered = c.renderThinking("first\nsecond")

	if strings.Contains(rendered, "> first") || !strings.Contains(rendered, "│") {
		t.Fatalf("thinking was not rendered as a quote: %q", rendered)
	}
	if !strings.Contains(rendered, "first") || !strings.Contains(rendered, "second") {
		t.Fatalf("thinking content was lost: %q", rendered)
	}
}

func TestFullScreenViewAndTranscriptScrolling(t *testing.T) {
	var c *client
	var index int
	var view string

	c = tuiClient(t)
	for index = 0; index < 30; index++ {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: "line"})
	}

	view = c.View()
	if lipgloss.Height(view) != c.height {
		t.Fatalf("full-screen view height = %d, want %d; banner=%d input=%d status=%d", lipgloss.Height(view), c.height,
			lipgloss.Height(c.banner()), lipgloss.Height(boxStyle.Width(c.width-2).Render(c.input.View())), lipgloss.Height(c.statusView()))
	}
	if !strings.Contains(view, "mininaru") {
		t.Fatal("full-screen view does not contain the banner")
	}

	c.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if c.view.AtBottom() {
		t.Fatal("page up did not move the transcript")
	}
	c.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !c.view.AtBottom() {
		t.Fatal("end did not return to the latest transcript line")
	}
}

func TestTranscriptKeepsScrollPositionWhenNewOutputArrives(t *testing.T) {
	var c *client
	var index int
	var offset int
	var view string

	c = tuiClient(t)
	for index = 0; index < 40; index++ {
		c.transcript = append(c.transcript, transcriptEntry{kind: transcriptNotice, content: "history"})
	}
	c.refreshViewport(true)
	c.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	offset = c.view.YOffset

	c.Update(chatDeltaMsg("new output"))
	if c.view.YOffset != offset {
		t.Fatalf("new output moved viewport from %d to %d", offset, c.view.YOffset)
	}
	if !c.newOutput {
		t.Fatal("new output was not announced while scrolled back")
	}
	c.Update(toolEventMsg(core.ToolEvent{Phase: core.ToolEventStarted, Name: "grep", Arguments: `{}`}))
	if c.view.YOffset != offset {
		t.Fatalf("tool output moved viewport from %d to %d", offset, c.view.YOffset)
	}

	view = c.View()
	if !strings.Contains(view, "new output") || !strings.Contains(view, "End to latest") {
		t.Fatalf("new output indicator missing: %q", view)
	}

	c.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if c.newOutput || !c.view.AtBottom() {
		t.Fatal("end did not clear the new output state")
	}
}

func TestResponsiveTUILayoutKeepsTheScreenBounds(t *testing.T) {
	var c *client
	var sizes []tea.WindowSizeMsg
	var size tea.WindowSizeMsg
	var view string

	c = tuiClient(t)
	c.transcript = append(c.transcript, transcriptEntry{kind: transcriptMessage, role: "assistant",
		content: "A response that wraps across the available terminal width without breaking the layout."})

	sizes = []tea.WindowSizeMsg{
		{Width: 40, Height: 12},
		{Width: 72, Height: 24},
		{Width: 120, Height: 40},
	}

	for _, size = range sizes {
		c.Update(size)
		view = c.View()

		if lipgloss.Height(view) != size.Height {
			t.Fatalf("%dx%d layout height = %d", size.Width, size.Height, lipgloss.Height(view))
		}
		if lipgloss.Width(view) > size.Width {
			t.Fatalf("%dx%d layout width = %d", size.Width, size.Height, lipgloss.Width(view))
		}
	}

	c.approval = &toolApprovalMsg{name: "bash_exec", arguments: strings.Repeat("command ", 20),
		response: make(chan approvalDecision, 1)}
	c.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	view = c.View()
	if lipgloss.Height(view) != 12 || lipgloss.Width(view) > 40 {
		t.Fatalf("narrow approval layout is %dx%d", lipgloss.Width(view), lipgloss.Height(view))
	}
}

func quitCmd(t *testing.T, c *client, text string) bool {
	var cmd tea.Cmd
	var msg tea.Msg
	var quit bool

	t.Helper()

	c.input.SetValue(text)
	_, cmd = c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		return false
	}

	msg = cmd()
	_, quit = msg.(tea.QuitMsg)

	return quit
}

func TestSlashExitAndQuitLeaveWithoutSending(t *testing.T) {
	var c *client

	c = tuiClient(t)
	if !quitCmd(t, c, "/exit") {
		t.Fatal("/exit did not quit")
	}
	if c.sending {
		t.Fatal("/exit was sent to the model")
	}
	if !c.quitting {
		t.Fatal("/exit did not mark the client as quitting")
	}

	c = tuiClient(t)
	if !quitCmd(t, c, "/quit") {
		t.Fatal("/quit did not quit")
	}
}

func TestSlashCompactStartsWorkAndIsNotSentToTheModel(t *testing.T) {
	var c *client
	var entry transcriptEntry

	c = tuiClient(t)
	typeEnter(c, "/compact")

	if !c.sending {
		t.Fatal("/compact did not start working")
	}
	if c.cancel == nil {
		t.Fatal("/compact left no way to interrupt it")
	}

	for _, entry = range c.transcript {
		if entry.kind == transcriptMessage && entry.role == "user" {
			t.Fatal("/compact was recorded as a user message")
		}
	}
}

func TestSlashCompactIsIgnoredWhileAnAnswerIsInFlight(t *testing.T) {
	var c *client

	c = tuiClient(t)
	c.sending = true

	typeEnter(c, "/compact")

	if c.cancel != nil {
		t.Fatal("/compact started while an answer was in flight")
	}
	if len(c.transcript) != 0 {
		t.Fatalf("transcript = %#v, want the key ignored like every other command in flight", c.transcript)
	}
}

func TestCompactOutcomeNotices(t *testing.T) {
	var c *client

	c = tuiClient(t)

	c.finishCompact(compactDoneMsg{compacted: true})
	if !strings.Contains(c.transcript[len(c.transcript)-1].content, "compacted the conversation") {
		t.Fatalf("success notice = %q", c.transcript[len(c.transcript)-1].content)
	}
	if !strings.Contains(c.transcript[len(c.transcript)-1].content, "refreshes after the next response") {
		t.Fatalf("success notice does not explain token refresh: %q", c.transcript[len(c.transcript)-1].content)
	}
	if c.sending {
		t.Fatal("finishCompact left the client sending")
	}

	c.finishCompact(compactDoneMsg{compacted: false})
	if !strings.Contains(c.transcript[len(c.transcript)-1].content, "nothing to compact") {
		t.Fatalf("empty notice = %q", c.transcript[len(c.transcript)-1].content)
	}
}

func TestHelpListsTheNewCommands(t *testing.T) {
	var c *client
	var body string

	c = tuiClient(t)
	typeEnter(c, "/help")

	body = c.transcript[len(c.transcript)-1].content
	if !strings.Contains(body, "/compact") || !strings.Contains(body, "/exit") ||
		!strings.Contains(body, "/usage") {
		t.Fatalf("help = %q", body)
	}
}

func TestSlashUsageReportsWithoutSending(t *testing.T) {
	var c *client
	var notice string

	c = tuiClient(t)
	typeEnter(c, "/usage")

	if c.sending {
		t.Fatal("/usage was sent to the model")
	}

	notice = c.transcript[len(c.transcript)-1].content
	if !strings.Contains(notice, "no token usage recorded") {
		t.Fatalf("usage notice = %q", notice)
	}
}
