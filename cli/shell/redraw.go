// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"os"
	"strings"

	"golang.org/x/term"
)

func termWidth() int {
	var cols int

	var err error

	cols, _, err = term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 {
		return 80
	}

	return cols
}

func stripAnsi(text string) string {
	var builder strings.Builder
	var runes []rune
	var i int

	runes = []rune(text)

	for i < len(runes) {
		if runes[i] == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			i = i + 2

			for i < len(runes) && !(runes[i] >= '@' && runes[i] <= '~') {
				i++
			}

			i++
			continue
		}

		builder.WriteRune(runes[i])
		i++
	}

	return builder.String()
}

func rowsFor(cols int, text string) int {
	var lines []string
	var one string
	var width int
	var rows int

	lines = strings.Split(stripAnsi(text), "\n")

	for _, one = range lines {
		width = displayWidth(one)
		if width == 0 {
			rows = rows + 1
			continue
		}

		rows = rows + (width+cols-1)/cols
	}

	return rows
}

func redraw(sh *state, before string, line []rune, pos int) {
	var cols int
	var oldRows int
	var trailing int

	cols = termWidth()
	oldRows = rowsFor(cols, prompt(sh)+before)

	if oldRows > 1 {
		write("\x1b[%dA", oldRows-1)
	}

	write("\r\x1b[0J%s%s", prompt(sh), string(line))

	trailing = displayWidth(string(line[pos:]))
	if trailing > 0 {
		write("\x1b[%dD", trailing)
	}
}
