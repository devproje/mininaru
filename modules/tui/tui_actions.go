// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devproje/mininaru/modules/client"
)

var toolPrimaryArg = map[string]string{
	"bash_exec":        "command",
	"file_read":        "path",
	"file_write":       "path",
	"file_edit":        "path",
	"browser_navigate": "url",
	"browser_click":    "selector",
	"browser_read":     "selector",
	"browser_type":     "selector",
}

func formatToolCall(name, arguments string) string {
	var field string
	var payload map[string]any
	var value any
	var text string
	var ok bool

	field, ok = toolPrimaryArg[name]
	if ok {
		if json.Unmarshal([]byte(arguments), &payload) == nil {
			value, ok = payload[field]
			if ok {
				text, ok = value.(string)
			}
		}
	}

	if text == "" {
		text = arguments
	}

	return fmt.Sprintf("%s(%s)", name, text)
}

func dimEachLine(text string) string {
	var lines []string
	var i int

	lines = strings.Split(text, "\n")
	for i = range lines {
		lines[i] = client.DIM + lines[i] + client.RESET
	}

	return strings.Join(lines, "\n")
}

func (m *tuiModel) pushHistory(value string) {
	if len(m.history) > 0 && m.history[len(m.history)-1] == value {
		return
	}

	m.history = append(m.history, value)
	m.historyPos = -1
	m.historyDraft = ""
}

func (m *tuiModel) newLine() {
	m.lines = append(m.lines, "")
}

func (m *tuiModel) appendText(text string) {
	var parts []string
	var i int

	if text == "" {
		return
	}

	parts = strings.Split(text, "\n")

	if len(m.lines) == 0 {
		m.lines = append(m.lines, "")
	}

	m.lines[len(m.lines)-1] += parts[0]

	for i = 1; i < len(parts); i++ {
		m.lines = append(m.lines, parts[i])
	}
}

func (m *tuiModel) appendStream(reasoning, content string) {
	if reasoning != "" {
		m.appendText(dimEachLine(reasoning))
	}

	if content != "" {
		m.appendText(m.md.Write(content))
	}
}

func (m *tuiModel) beginAsync(label string) tea.Cmd {
	m.newLine()
	m.appendText(fmt.Sprintf("%s %srunning /%s...%s", client.StatusDot("started"), client.DIM, label, client.RESET))
	m.activeToolLine = len(m.lines) - 1
	m.activeToolName = label
	m.newLine()
	m.busy = true

	return spinTickCmd()
}

func (m *tuiModel) landAsyncResult(label string, lines []string) {
	var replaced bool
	var i int

	replaced = m.activeToolLine >= 0 && m.activeToolLine < len(m.lines) && m.activeToolName == label

	if len(lines) == 0 && replaced {
		m.lines[m.activeToolLine] = fmt.Sprintf("%s %s/%s%s done", client.StatusDot("finished"), client.WHITE, label, client.RESET)
	}

	for i = range lines {
		if i == 0 && replaced {
			m.lines[m.activeToolLine] = lines[0]
			continue
		}

		m.newLine()
		m.appendText(lines[i])
	}

	m.activeToolLine = -1
	m.activeToolName = ""
	m.busy = false
	m.newLine()
}

func toolSummary(name, status, message string) string {
	var added, removed int

	if status == "started" {
		return ""
	}

	if client.LooksLikeDiff(message) {
		added, removed = client.DiffStat(message)

		return fmt.Sprintf("%s %s%s%s %s  %s+%d %s-%d%s", client.StatusDot(status), client.WHITE, name, client.RESET, status, client.GREEN, added, client.RED, removed, client.RESET)
	}

	if message == "" {
		return fmt.Sprintf("%s %s%s%s %s", client.StatusDot(status), client.WHITE, name, client.RESET, status)
	}

	if strings.Contains(message, "\n") {
		return fmt.Sprintf("%s %s%s%s %s", client.StatusDot(status), client.WHITE, name, client.RESET, status)
	}

	return fmt.Sprintf("%s %s%s%s %s  %s%s%s", client.StatusDot(status), client.WHITE, name, client.RESET, status, client.DIM, message, client.RESET)
}

func (m *tuiModel) sendMessage(value string) tea.Cmd {
	m.lines = append(m.lines, tuiUserPrefix.Render("> "+value), "")
	m.busy = true
	m.streamMode = ""
	m.layout()

	m.conn.WriteJSON(client.Frame{SessionId: m.session, Content: value, Cwd: m.cwd, NoCache: m.noCache, Images: m.pending})
	m.pending = nil
	m.imageCount = 0

	return spinTickCmd()
}

func (m *tuiModel) queueImage(path string, cleanup bool) tea.Cmd {
	m.imageCount++
	m.input.InsertString(fmt.Sprintf("[image #%d] ", m.imageCount))

	return uploadImageCmd(m.base, m.apiKey, m.session, path, cleanup)
}

func (m *tuiModel) flushQueued() tea.Cmd {
	var next string

	if len(m.queuedMsgs) == 0 || m.busy || m.conn == nil {
		return nil
	}

	next, m.queuedMsgs = m.queuedMsgs[0], m.queuedMsgs[1:]

	return m.sendMessage(next)
}

func (m tuiModel) answerCmd(value string) tea.Cmd {
	var answers chan string

	answers = m.answers

	return func() tea.Msg {
		answers <- value

		return tuiAnsweredMsg{}
	}
}

func (m tuiModel) submit() (tea.Model, tea.Cmd) {
	var value string
	var suggestions []tuiCmdInfo

	value = strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}

	m.input.Reset()
	m.pushHistory(value)

	if m.awaiting != "" {
		return m, m.answerCmd(value)
	}

	if strings.HasPrefix(value, "/") {
		suggestions = tuiCmdSuggestions(value)
		if len(suggestions) > 0 && !tuiCmdExact(value) {
			if m.cmdCursor >= len(suggestions) {
				m.cmdCursor = 0
			}

			value = "/" + suggestions[m.cmdCursor].name
		}

		return m.runCommand(value)
	}

	if m.conn == nil {
		m.lines = append(m.lines, tuiUserPrefix.Render("> "+value), "")
		m.layout()

		return m, nil
	}

	if m.busy {
		m.queuedMsgs = append(m.queuedMsgs, value)
		m.lines = append(m.lines, tuiUserPrefix.Render("> "+value), fmt.Sprintf("%s⏸ queued — sends once the current turn finishes%s", client.DIM, client.RESET), "")
		m.layout()

		return m, nil
	}

	return m, m.sendMessage(value)
}
