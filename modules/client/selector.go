// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"fmt"
)

const menuRows int = 10

func menuWindow(count int, cursor int) (int, int) {
	var start int

	if count <= menuRows {
		return 0, count
	}

	start = cursor - menuRows/2
	if start < 0 {
		start = 0
	}
	if start > count-menuRows {
		start = count - menuRows
	}

	return start, start + menuRows
}

func drawMenu(title string, items []string, cursor int, redraw bool) {
	var start int
	var end int
	var i int

	start, end = menuWindow(len(items), cursor)

	if redraw {
		write("\x1b[%dA", end-start+1)
	}

	write("\r\x1b[0J%s%s%s\n", DIM, title, RESET)

	for i = start; i < end; i++ {
		if i == cursor {
			write("%s❯ %s%s\n", PURPLE, items[i], RESET)
			continue
		}

		write("  %s\n", items[i])
	}
}

func clearMenu(count int, cursor int) {
	var start int
	var end int

	start, end = menuWindow(count, cursor)

	write("\x1b[%dA\r\x1b[0J", end-start+1)
}

func selectFrom(title string, items []string, stream keys) (int, error) {
	var cursor int
	var b byte
	var final byte

	var err error

	if len(items) == 0 {
		return -1, fmt.Errorf("nothing to choose from")
	}

	drawMenu(title, items, cursor, false)

	for {
		b, err = stream.next()
		if err != nil {
			return -1, err
		}

		switch b {
		case '\r', '\n':
			clearMenu(len(items), cursor)

			return cursor, nil
		case 0x03:
			clearMenu(len(items), cursor)

			return -1, errInterrupted
		case 'k':
			if cursor > 0 {
				cursor--
			}
		case 'j':
			if cursor < len(items)-1 {
				cursor++
			}
		case 0x1b:
			_, final = stream.escape()

			switch final {
			case 0:
				clearMenu(len(items), cursor)

				return -1, errInterrupted
			case 'A':
				if cursor > 0 {
					cursor--
				}
			case 'B':
				if cursor < len(items)-1 {
					cursor++
				}
			}
		default:
			continue
		}

		drawMenu(title, items, cursor, true)
	}
}
