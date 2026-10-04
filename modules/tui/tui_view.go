// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/devproje/mininaru/modules/client"
)

func (m *tuiModel) footerHeight() int {
	var height int
	var suggestions []tuiCmdInfo

	height = tuiFooterHeight

	if m.busy && m.awaiting == "" {
		height++
	}

	if m.awaiting != "" && len(m.awaitOptions) > 0 {
		height += len(m.awaitOptions)
	}

	if m.awaiting == "" {
		suggestions = tuiCmdSuggestions(m.input.Value())
		if len(suggestions) > 0 && !tuiCmdExact(m.input.Value()) {
			height += min(len(suggestions), maxCmdSuggestionRows)
		}
	}

	return height
}

func suggestionWindow(cursor, total, max int) (int, int) {
	var start int

	if total <= max {
		return 0, total
	}

	start = cursor - max + 1
	if start < 0 {
		start = 0
	}
	if start > total-max {
		start = total - max
	}

	return start, start + max
}

func (m *tuiModel) layout() {
	var footerH int
	var header string
	var body string
	var content string

	if m.width <= 0 || m.height <= 0 {
		return
	}

	footerH = m.footerHeight()

	if !m.ready {
		m.viewport = viewport.New(m.width, m.height-footerH)
		m.ready = true
	}

	if len(m.lines) > 0 {
		m.lines[0] = tuiHeaderBlock(m.agent, m.session, client.ModeColor(m.mode), m.width)
		header = m.lines[0]
	}

	if len(m.lines) > 1 {
		body = strings.Join(m.lines[1:], "\n")
	}

	if m.width > 0 {
		body = lipgloss.NewStyle().Width(m.width).Padding(0, 1).Render(body)
	}

	content = header
	if body != "" {
		content = header + "\n" + body
	}

	m.viewport.Width = m.width
	m.viewport.Height = m.height - footerH
	m.input.SetWidth(m.width - tuiInputBoxStyle(client.ModeColorCode(m.mode)).GetHorizontalFrameSize())
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m tuiModel) awaitMenu(indent int) string {
	var i int
	var label string
	var line string
	var lines []string

	for i, label = range m.awaitLabels {
		if i == m.awaitCursor {
			line = fmt.Sprintf("%s❯ %s%s", client.PURPLE, label, client.RESET)
		} else {
			line = fmt.Sprintf("  %s%s%s", client.DIM, label, client.RESET)
		}

		lines = append(lines, line)
	}

	return lipgloss.NewStyle().PaddingLeft(indent).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) cmdMenu(indent int, suggestions []tuiCmdInfo) string {
	var start int
	var end int
	var i int
	var c tuiCmdInfo
	var line string
	var lines []string

	start, end = suggestionWindow(m.cmdCursor, len(suggestions), maxCmdSuggestionRows)

	for i = start; i < end; i++ {
		c = suggestions[i]
		if i == m.cmdCursor {
			line = fmt.Sprintf("%s❯ /%s%s  %s%s%s", client.PURPLE, c.name, client.RESET, client.DIM, c.desc, client.RESET)
		} else {
			line = fmt.Sprintf("  %s/%s%s  %s%s%s", client.DIM, c.name, client.RESET, client.DIM, c.desc, client.RESET)
		}

		lines = append(lines, line)
	}

	return lipgloss.NewStyle().PaddingLeft(indent).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) footer() string {
	var inputStyle lipgloss.Style
	var indent int
	var parts []string
	var suggestions []tuiCmdInfo
	var box string
	var badge string
	var hintText string
	var hint string

	inputStyle = tuiInputBoxStyle(client.ModeColorCode(m.mode))
	indent = inputStyle.GetBorderLeftSize() + inputStyle.GetPaddingLeft()

	if m.busy && m.awaiting == "" {
		parts = append(parts, lipgloss.NewStyle().PaddingLeft(indent).Render(
			fmt.Sprintf("%s%s%s %sworking...%s", client.PURPLE, client.BarFrame(m.spinTick), client.RESET, client.DIM, client.RESET)))
	}

	if m.awaiting != "" && len(m.awaitOptions) > 0 {
		parts = append(parts, m.awaitMenu(indent))
	}

	if m.awaiting == "" {
		suggestions = tuiCmdSuggestions(m.input.Value())
		if len(suggestions) > 0 && !tuiCmdExact(m.input.Value()) {
			parts = append(parts, m.cmdMenu(indent, suggestions))
		}
	}

	box = inputStyle.Width(m.width - inputStyle.GetHorizontalBorderSize()).Render(m.input.View())
	parts = append(parts, box)

	badge = fmt.Sprintf("%s%s%s", client.ModeColor(m.mode), strings.ToUpper(m.mode), client.RESET)

	switch {
	case m.awaiting != "" && len(m.awaitOptions) > 0:
		hintText = "up/down to select  ·  enter to confirm"
	case m.awaiting == "question":
		hintText = "type an answer, then enter"
	case len(suggestions) > 0 && !tuiCmdExact(m.input.Value()):
		hintText = "up/down to select  ·  enter to run  ·  tab to complete"
	case !m.input.Focused():
		hintText = "up/down to scroll  ·  esc to type"
	case m.busy:
		hintText = "ctrl+c interrupt"
	default:
		hintText = "shift+tab mode  ·  ctrl+c exit"
	}

	hint = lipgloss.NewStyle().PaddingLeft(indent).Render(badge + "  ·  " + tuiHint.Render(hintText))
	parts = append(parts, hint)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m tuiModel) View() string {
	if !m.ready {
		return "initializing...\r\n"
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		m.footer(),
	)
}
