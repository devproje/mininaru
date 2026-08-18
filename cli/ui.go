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
	dimColor    = lipgloss.Color("#8a8178")
	errorColor  = lipgloss.Color("#c81c1c")
)

var (
	metaStyle   = lipgloss.NewStyle().Foreground(dimColor)
	hintStyle   = lipgloss.NewStyle().Foreground(dimColor)
	statusStyle = lipgloss.NewStyle().Foreground(accentColor)
	errStyle    = lipgloss.NewStyle().Foreground(errorColor)
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
