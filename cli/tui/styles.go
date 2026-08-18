// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "github.com/charmbracelet/lipgloss"

const (
	accentColor = lipgloss.Color("#f0b060")
	userColor   = lipgloss.Color("#e8e8e8")
	blushColor  = lipgloss.Color("#f0b8a8")
	dimColor    = lipgloss.Color("#8a8178")
	errorColor  = lipgloss.Color("#c81c1c")
	addedColor  = lipgloss.Color("#70b070")
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
	toolBodyStyle  = lipgloss.NewStyle().Foreground(dimColor).BorderStyle(lipgloss.NormalBorder()).BorderLeft(true).PaddingLeft(1)
	toolAddedStyle = lipgloss.NewStyle().Foreground(addedColor)
	toolCutStyle   = lipgloss.NewStyle().Foreground(errorColor)
)
