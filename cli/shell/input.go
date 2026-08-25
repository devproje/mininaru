// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"io"
	"os"
	"unicode/utf8"
)

func skipEscape(input *os.File, buf []byte) bool {
	var count int

	var err error

	count, err = input.Read(buf[:1])
	if err != nil || count == 0 {
		return false
	}

	if buf[0] != '[' {
		return false
	}

	for {
		count, err = input.Read(buf[:1])
		if err != nil || count == 0 {
			return false
		}

		if buf[0] == 'Z' {
			return true
		}

		if buf[0] >= '@' && buf[0] <= '~' {
			return false
		}
	}
}

func readLine(sh *state) (string, error) {
	var buf []byte
	var line []rune
	var count int
	var pending []byte
	var letter rune
	var completed string
	var listed bool
	var repeated bool

	var err error

	buf = make([]byte, 1)
	write("%s", prompt(sh))

	for {
		count, err = os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}

		if count == 0 {
			continue
		}

		if buf[0] != '\t' {
			repeated = false
		}

		switch buf[0] {
		case '\t':
			completed, listed = complete(sh, string(line), repeated)
			line = []rune(completed)
			repeated = listed

			write("\r\x1b[2K%s%s", prompt(sh), string(line))
			continue
		case 0x03:
			write("%s^C%s\n", GRAY, RESET)
			line = nil
			write("%s", prompt(sh))
			continue
		case 0x04:
			return "", io.EOF
		case 0x15:
			write("\r\x1b[2K%s", prompt(sh))
			line = nil
			continue
		case '\r', '\n':
			write("\n")
			return string(line), nil
		case 0x7f, 0x08:
			if len(line) == 0 {
				continue
			}

			line = line[:len(line)-1]
			write("\r\x1b[2K%s%s", prompt(sh), string(line))
			continue
		case 0x1b:
			if !skipEscape(os.Stdin, buf) {
				continue
			}

			toggleMode(sh)
			write("\r\x1b[2K%s%s", prompt(sh), string(line))
			continue
		}

		if buf[0] < 0x20 {
			continue
		}

		pending = append(pending, buf[0])

		if !utf8.FullRune(pending) && len(pending) < utf8.UTFMax {
			continue
		}

		letter, _ = utf8.DecodeRune(pending)
		pending = pending[:0]

		if letter == utf8.RuneError {
			continue
		}

		line = append(line, letter)
		write("%s", string(letter))
	}
}
