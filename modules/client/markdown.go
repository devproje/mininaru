// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"fmt"
	"strings"
)

type mdRenderer struct {
	buf     string
	table   []string
	inFence bool
}

type mdLink struct {
	text  string
	url   string
	width int
	ok    bool
}

func parseLink(s string) mdLink {
	var bracket int
	var paren int
	var end int

	bracket = strings.IndexByte(s, ']')
	if bracket < 0 || bracket+1 >= len(s) || s[bracket+1] != '(' {
		return mdLink{}
	}

	paren = bracket + 1
	end = strings.IndexByte(s[paren:], ')')
	if end < 0 {
		return mdLink{}
	}

	return mdLink{
		text:  s[1:bracket],
		url:   s[paren+1 : paren+end],
		width: paren + end + 1,
		ok:    true,
	}
}

func inlineMarkdown(s string) string {
	var out string
	var marker string
	var link mdLink
	var end int
	var i int

	for i = 0; i < len(s); i++ {
		if s[i] == '`' {
			end = strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				out = fmt.Sprintf("%s%s%s%s", out, RED, s[i+1:i+1+end], RESET)
				i = i + end + 1
				continue
			}
		}

		if strings.HasPrefix(s[i:], "**") || strings.HasPrefix(s[i:], "__") {
			marker = s[i : i+2]
			end = strings.Index(s[i+2:], marker)
			if end >= 0 {
				out = fmt.Sprintf("%s%s%s%s", out, BOLD, s[i+2:i+2+end], RESET)
				i = i + end + 3
				continue
			}
		}

		if s[i] == '*' || s[i] == '_' {
			end = strings.IndexByte(s[i+1:], s[i])
			if end > 0 {
				out = fmt.Sprintf("%s\x1b[3m%s\x1b[23m", out, s[i+1:i+1+end])
				i = i + end + 1
				continue
			}
		}

		if s[i] == '[' {
			link = parseLink(s[i:])
			if link.ok {
				out = fmt.Sprintf("%s\x1b[4m%s\x1b[24m%s (%s)%s", out, link.text, DIM, link.url, RESET)
				i = i + link.width - 1
				continue
			}
		}

		out = fmt.Sprintf("%s%s", out, s[i:i+1])
	}

	return out
}

func headingLevel(trimmed string) int {
	var i int

	for i = 0; i < len(trimmed) && i < 6; i++ {
		if trimmed[i] != '#' {
			break
		}
	}

	if i > 0 && i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
		return i
	}

	return 0
}

func isThematicBreak(trimmed string) bool {
	var ch byte
	var count int
	var r rune

	if len(trimmed) < 3 {
		return false
	}

	ch = trimmed[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}

	for _, r = range trimmed {
		if r == ' ' {
			continue
		}
		if byte(r) != ch {
			return false
		}
		count++
	}

	return count >= 3
}

func blockquoteBody(trimmed string) string {
	if !strings.HasPrefix(trimmed, ">") {
		return ""
	}

	return strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " ")
}

func listItem(trimmed string) (string, bool) {
	var i int

	if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
		return strings.TrimSpace(trimmed[2:]), true
	}

	for i = 0; i < len(trimmed); i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			break
		}
	}

	if i > 0 && i+1 < len(trimmed) && (trimmed[i] == '.' || trimmed[i] == ')') && trimmed[i+1] == ' ' {
		return strings.TrimSpace(trimmed[i+2:]), true
	}

	return "", false
}

func isTableRow(trimmed string) bool {
	return len(trimmed) >= 2 && strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

func splitCells(row string) []string {
	var cells []string
	var i int

	cells = strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")

	for i = 0; i < len(cells); i++ {
		cells[i] = strings.TrimSpace(cells[i])
	}

	return cells
}

func isSeparatorCell(cell string) bool {
	var i int

	cell = strings.TrimSuffix(strings.TrimPrefix(cell, ":"), ":")

	for i = 0; i < len(cell); i++ {
		if cell[i] != '-' {
			return false
		}
	}

	return len(cell) > 0
}

func isTableSeparator(trimmed string) bool {
	var cells []string
	var cell string

	cells = splitCells(trimmed)
	if len(cells) == 0 {
		return false
	}

	for _, cell = range cells {
		if !isSeparatorCell(cell) {
			return false
		}
	}

	return true
}

func cellAligns(sep string) []int {
	var cells []string
	var aligns []int
	var i int
	var cell string

	cells = splitCells(sep)
	aligns = make([]int, len(cells))

	for i = 0; i < len(cells); i++ {
		cell = cells[i]

		if strings.HasPrefix(cell, ":") && strings.HasSuffix(cell, ":") {
			aligns[i] = 2
			continue
		}

		if strings.HasSuffix(cell, ":") {
			aligns[i] = 1
		}
	}

	return aligns
}

func padCell(text string, width int, align int) string {
	var gap int

	gap = width - displayWidth(stripAnsi(text))
	if gap <= 0 {
		return text
	}

	switch align {
	case 1:
		return strings.Repeat(" ", gap) + text
	case 2:
		return strings.Repeat(" ", gap/2) + text + strings.Repeat(" ", gap-gap/2)
	}

	return text + strings.Repeat(" ", gap)
}

func renderTable(rows []string) string {
	var aligns []int
	var cells []string
	var grid [][]string
	var cols int
	var widths []int
	var cell string
	var span int
	var align int
	var out strings.Builder
	var r int
	var c int

	aligns = cellAligns(rows[1])

	for r = 0; r < len(rows); r++ {
		if r == 1 {
			continue
		}

		cells = splitCells(rows[r])
		grid = append(grid, cells)

		if len(cells) > cols {
			cols = len(cells)
		}
	}

	widths = make([]int, cols)

	for r = 0; r < len(grid); r++ {
		for c = 0; c < cols && c < len(grid[r]); c++ {
			grid[r][c] = inlineMarkdown(grid[r][c])

			span = displayWidth(stripAnsi(grid[r][c]))
			if span > widths[c] {
				widths[c] = span
			}
		}
	}

	for r = 0; r < len(grid); r++ {
		out.WriteString("  ")

		for c = 0; c < cols; c++ {
			cell = ""
			if c < len(grid[r]) {
				cell = grid[r][c]
			}

			if r == 0 {
				cell = BOLD + cell + RESET
			}

			align = 0
			if c < len(aligns) {
				align = aligns[c]
			}

			out.WriteString(padCell(cell, widths[c], align))

			if c < cols-1 {
				out.WriteString("  ")
			}
		}

		out.WriteString("\n")

		if r != 0 {
			continue
		}

		out.WriteString("  ")

		for c = 0; c < cols; c++ {
			out.WriteString(DIM + strings.Repeat("─", widths[c]) + RESET)

			if c < cols-1 {
				out.WriteString("  ")
			}
		}

		out.WriteString("\n")
	}

	return out.String()
}

func (m *mdRenderer) formatLine(line string) string {
	var trimmed string
	var indent string
	var body string
	var hashes int
	var ok bool

	trimmed = strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
		m.inFence = !m.inFence
		if m.inFence && len(trimmed) > 3 {
			return GRAY + "│ " + RESET + DIM + strings.TrimSpace(trimmed[3:]) + RESET
		}
		return GRAY + "╌╌╌" + RESET
	}

	if m.inFence {
		return GRAY + "│ " + RESET + line
	}

	if line == "" {
		return ""
	}

	if isThematicBreak(trimmed) {
		return DIM + strings.Repeat("─", 40) + RESET
	}

	hashes = headingLevel(trimmed)
	if hashes > 0 {
		return BOLD + PURPLE + strings.TrimSpace(trimmed[hashes:]) + RESET
	}

	indent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]

	body = blockquoteBody(trimmed)
	if body != "" || trimmed == ">" {
		return indent + DIM + "▏ " + RESET + inlineMarkdown(body)
	}

	body, ok = listItem(trimmed)
	if ok {
		return indent + PURPLE + "• " + RESET + inlineMarkdown(body)
	}

	return indent + inlineMarkdown(strings.TrimLeft(line, " \t"))
}

func (m *mdRenderer) drainTable() string {
	var rows []string
	var out string
	var row string

	if len(m.table) == 0 {
		return ""
	}

	rows = m.table
	m.table = nil

	if len(rows) < 2 || !isTableSeparator(rows[1]) {
		for _, row = range rows {
			out = out + m.formatLine(row) + "\n"
		}

		return out
	}

	return renderTable(rows)
}

func (m *mdRenderer) line(raw string) string {
	var trimmed string

	trimmed = strings.TrimSpace(raw)

	if !m.inFence && isTableRow(trimmed) {
		m.table = append(m.table, trimmed)

		return ""
	}

	return m.drainTable() + m.formatLine(raw) + "\n"
}

func (m *mdRenderer) write(delta string) string {
	var out string
	var i int

	for i = 0; i < len(delta); i++ {
		if delta[i] != '\n' {
			m.buf = fmt.Sprintf("%s%s", m.buf, delta[i:i+1])
			continue
		}

		out = out + m.line(m.buf)
		m.buf = ""
	}

	return out
}

func (m *mdRenderer) flush() string {
	var out string
	var line string

	out = m.drainTable()

	if m.buf == "" {
		return strings.TrimRight(out, "\n")
	}

	line = m.buf
	m.buf = ""

	return out + m.formatLine(line)
}

func (m *mdRenderer) reset() {
	m.buf = ""
	m.table = nil
	m.inFence = false
}
