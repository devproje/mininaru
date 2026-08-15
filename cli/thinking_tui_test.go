// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

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

func TestToolApprovalKeys(t *testing.T) {
	var c *client
	var response chan bool
	var approved bool

	c = tuiClient(t)
	c.sending = true
	response = make(chan bool, 1)
	c.Update(toolApprovalMsg{name: "bash_exec", arguments: `{"command":"pwd"}`, response: response})
	if c.approval == nil || !strings.Contains(c.statusView(), "approve bash_exec") {
		t.Fatal("tool approval prompt was not displayed")
	}

	c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	approved = <-response
	if !approved || c.approval != nil {
		t.Fatal("y did not approve and clear the request")
	}

	response = make(chan bool, 1)
	c.Update(toolApprovalMsg{name: "file_write", arguments: `{}`, response: response})
	c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	approved = <-response
	if approved || c.approval != nil {
		t.Fatal("n did not deny and clear the request")
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
	if c.scrollOffset == 0 {
		t.Fatal("page up did not move the transcript")
	}
	c.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if c.scrollOffset != 0 {
		t.Fatal("end did not return to the latest transcript line")
	}
}

func quitCmd(t *testing.T, c *client, text string) bool {
	var cmd tea.Cmd
	var msg tea.Msg

	t.Helper()

	c.input.SetValue(text)
	_, cmd = c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		return false
	}

	msg = cmd()
	_, quit := msg.(tea.QuitMsg)

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
