// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
)

type uiRows struct {
	buffer bytes.Buffer
	writer *tabwriter.Writer
}

const (
	accentColor = lipgloss.Color("#f0b060")
	userColor   = lipgloss.Color("#e8e8e8")
	blushColor  = lipgloss.Color("#f0b8a8")
	dimColor    = lipgloss.Color("#8a8178")
	errorColor  = lipgloss.Color("#c81c1c")
)

var (
	bannerStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	metaStyle   = lipgloss.NewStyle().Foreground(dimColor)

	userMarkStyle = lipgloss.NewStyle().Foreground(userColor).Bold(true)
	userTextStyle = lipgloss.NewStyle().Foreground(dimColor)
	naruMarkStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	naruTextStyle = lipgloss.NewStyle()

	boxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dimColor).Padding(0, 1)
	boxBusyStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accentColor).Padding(0, 1)

	hintStyle   = lipgloss.NewStyle().Foreground(dimColor)
	statusStyle = lipgloss.NewStyle().Foreground(accentColor)
	errStyle    = lipgloss.NewStyle().Foreground(errorColor)

	thinkMarkStyle = lipgloss.NewStyle().Foreground(blushColor)
	thinkTextStyle = lipgloss.NewStyle().Foreground(blushColor).Italic(true)
	toolMarkStyle  = lipgloss.NewStyle().Foreground(accentColor)
)

func uiTable(headers ...string) *uiRows {
	var rows *uiRows

	rows = &uiRows{}
	rows.writer = tabwriter.NewWriter(&rows.buffer, 0, 0, 3, ' ', 0)

	rows.row(headers...)

	return rows
}

func (r *uiRows) row(cells ...string) {
	fmt.Fprintln(r.writer, strings.Join(cells, "\t"))
}

func (r *uiRows) flush() {
	var lines []string
	var index int

	r.writer.Flush()

	lines = strings.Split(strings.TrimRight(r.buffer.String(), "\n"), "\n")

	for index = range lines {
		lines[index] = strings.TrimRight(lines[index], " ")
	}

	lines[0] = metaStyle.Render(lines[0])

	for index = range lines {
		fmt.Fprintln(os.Stdout, lines[index])
	}
}

func uiEmpty(format string, args ...any) {
	fmt.Fprintln(os.Stderr, hintStyle.Render(fmt.Sprintf(format, args...)))
}

func uiOk(format string, args ...any) {
	fmt.Fprintln(os.Stdout, fmt.Sprintf(format, args...))
}

func uiNote(format string, args ...any) {
	fmt.Fprintln(os.Stderr, hintStyle.Render(fmt.Sprintf(format, args...)))
}
