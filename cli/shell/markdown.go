// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import "strings"

type mdRenderer struct {
	buf     strings.Builder
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
	var out strings.Builder
	var marker string
	var link mdLink
	var end int
	var i int

	for i = 0; i < len(s); i++ {
		if s[i] == '`' {
			end = strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				out.WriteString("\x1b[7m " + s[i+1:i+1+end] + " \x1b[27m")
				i = i + end + 1
				continue
			}
		}

		if strings.HasPrefix(s[i:], "**") || strings.HasPrefix(s[i:], "__") {
			marker = s[i : i+2]
			end = strings.Index(s[i+2:], marker)
			if end >= 0 {
				out.WriteString(BOLD + s[i+2:i+2+end] + RESET)
				i = i + end + 3
				continue
			}
		}

		if s[i] == '*' || s[i] == '_' {
			end = strings.IndexByte(s[i+1:], s[i])
			if end > 0 {
				out.WriteString("\x1b[3m" + s[i+1:i+1+end] + "\x1b[23m")
				i = i + end + 1
				continue
			}
		}

		if s[i] == '[' {
			link = parseLink(s[i:])
			if link.ok {
				out.WriteString("\x1b[4m" + link.text + "\x1b[24m" + DIM + " (" + link.url + ")" + RESET)
				i = i + link.width - 1
				continue
			}
		}

		out.WriteByte(s[i])
	}

	return out.String()
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

func (m *mdRenderer) write(delta string) string {
	var out strings.Builder
	var i int

	for i = 0; i < len(delta); i++ {
		if delta[i] != '\n' {
			m.buf.WriteByte(delta[i])
			continue
		}

		out.WriteString(m.formatLine(m.buf.String()))
		out.WriteByte('\n')
		m.buf.Reset()
	}

	return out.String()
}

func (m *mdRenderer) flush() string {
	var line string

	if m.buf.Len() == 0 {
		return ""
	}

	line = m.buf.String()
	m.buf.Reset()

	return m.formatLine(line)
}

func (m *mdRenderer) reset() {
	m.buf.Reset()
	m.inFence = false
}
