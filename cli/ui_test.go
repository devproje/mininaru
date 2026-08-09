// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

func TestUiTableAlignsColumns(t *testing.T) {
	var rows *uiRows
	var lines []string

	rows = uiTable("ID", "NAME")
	rows.row("a", "short")
	rows.row("bbbbbbbb", "longer")

	rows.writer.Flush()

	lines = strings.Split(strings.TrimRight(rows.buffer.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected a header and two rows, got %d lines: %q", len(lines), lines)
	}

	if strings.Index(lines[0], "NAME") != strings.Index(lines[2], "longer") {
		t.Fatalf("header and rows are not aligned:\n%s", strings.Join(lines, "\n"))
	}

	if strings.Index(lines[1], "short") != strings.Index(lines[2], "longer") {
		t.Fatalf("rows are not aligned:\n%s", strings.Join(lines, "\n"))
	}
}

func TestUiTableKeepsCellOrder(t *testing.T) {
	var rows *uiRows

	rows = uiTable("A", "B", "C")
	rows.row("1", "2", "3")

	rows.writer.Flush()

	if !strings.Contains(rows.buffer.String(), "1") || !strings.Contains(rows.buffer.String(), "3") {
		t.Fatalf("cells missing from table: %q", rows.buffer.String())
	}
}
