// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"errors"
	"io"
	"os"
	"time"
	"unicode"
	"unicode/utf8"
)

const escapeTimeout time.Duration = 50 * time.Millisecond
const idlePollInterval time.Duration = 100 * time.Millisecond
const shiftEnterParams string = "13;2"
const ctrlArrowParams string = "1;5"
const homeKeyParams string = "1"
const endKeyParams string = "4"

var errSoftNewline error = errors.New("soft newline")

func readByte(sh *state, buf []byte) (int, error) {
	if len(sh.pendingInput) > 0 {
		buf[0] = sh.pendingInput[0]
		sh.pendingInput = sh.pendingInput[1:]

		return 1, nil
	}

	return os.Stdin.Read(buf)
}

func readEscapeByte(sh *state, buf []byte) (int, error) {
	if len(sh.pendingInput) > 0 {
		return readByte(sh, buf)
	}

	if !pollStdin(escapeTimeout) {
		return 0, nil
	}

	return os.Stdin.Read(buf)
}

func readEscape(sh *state, buf []byte) (string, byte) {
	var count int
	var params []byte

	var err error

	count, err = readEscapeByte(sh, buf)
	if err != nil || count == 0 {
		return "", 0
	}

	if buf[0] != '[' {
		return "", 0
	}

	for {
		count, err = readEscapeByte(sh, buf)
		if err != nil || count == 0 {
			return "", 0
		}

		if buf[0] >= '@' && buf[0] <= '~' {
			return string(params), buf[0]
		}

		params = append(params, buf[0])
	}
}

func wordBoundaryLeft(line []rune, pos int) int {
	var i int

	i = pos

	for i > 0 && unicode.IsSpace(line[i-1]) {
		i--
	}

	for i > 0 && !unicode.IsSpace(line[i-1]) {
		i--
	}

	return i
}

func wordBoundaryRight(line []rune, pos int) int {
	var i int

	i = pos

	for i < len(line) && unicode.IsSpace(line[i]) {
		i++
	}

	for i < len(line) && !unicode.IsSpace(line[i]) {
		i++
	}

	return i
}

func readLine(sh *state) (string, error) {
	var buf []byte
	var line []rune
	var pos int
	var count int
	var pending []byte
	var letter rune
	var killFrom int
	var completed string
	var listed bool
	var repeated bool
	var before string
	var params string
	var final byte
	var histPos int
	var draft string
	var mirrored bool
	var reconnected bool

	var err error

	buf = make([]byte, 1)
	histPos = len(historyFor(sh))
	write("%s", prompt(sh))

	for {
		if len(sh.pendingInput) == 0 && !pollStdin(idlePollInterval) {
			mirrored = drainMirror(sh)
			reconnected = retryConnect(sh)

			if mirrored || reconnected {
				redraw(sh, "", line, pos)
			}

			continue
		}

		count, err = readByte(sh, buf)
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
			before = string(line)
			completed, listed = complete(sh, string(line), repeated)
			line = []rune(completed)
			pos = len(line)
			repeated = listed

			if listed {
				write("\r\x1b[2K%s%s", prompt(sh), string(line))
			} else {
				redraw(sh, before, line, pos)
			}
			continue
		case 0x01:
			if pos > 0 {
				pos = 0
				redraw(sh, string(line), line, pos)
			}
			continue
		case 0x03:
			write("%s^C%s\n", GRAY, RESET)

			if sh.continuation {
				return "", errContinuationCanceled
			}

			line = nil
			pos = 0
			write("%s", prompt(sh))
			continue
		case 0x04:
			return "", io.EOF
		case 0x05:
			if pos < len(line) {
				pos = len(line)
				redraw(sh, string(line), line, pos)
			}
			continue
		case 0x0b:
			if pos == len(line) {
				continue
			}

			sh.killBuffer = append([]rune{}, line[pos:]...)
			before = string(line)
			line = line[:pos]
			redraw(sh, before, line, pos)
			continue
		case 0x0c:
			write("\x1b[H\x1b[2J")
			redraw(sh, "", line, pos)
			continue
		case 0x15:
			if pos == 0 {
				continue
			}

			sh.killBuffer = append([]rune{}, line[:pos]...)
			before = string(line)
			line = append([]rune{}, line[pos:]...)
			pos = 0
			redraw(sh, before, line, pos)
			continue
		case 0x17:
			if pos == 0 {
				continue
			}

			killFrom = wordBoundaryLeft(line, pos)
			sh.killBuffer = append([]rune{}, line[killFrom:pos]...)
			before = string(line)
			line = append(line[:killFrom:killFrom], line[pos:]...)
			pos = killFrom
			redraw(sh, before, line, pos)
			continue
		case 0x19:
			if len(sh.killBuffer) == 0 {
				continue
			}

			before = string(line)
			line = append(line[:pos:pos], append(append([]rune{}, sh.killBuffer...), line[pos:]...)...)
			pos = pos + len(sh.killBuffer)
			redraw(sh, before, line, pos)
			continue
		case '\r':
			write("\n")
			return string(line), nil
		case '\n':
			write("\n")
			return string(line), errSoftNewline
		case 0x7f, 0x08:
			if pos == 0 {
				continue
			}

			before = string(line)
			line = append(line[:pos-1], line[pos:]...)
			pos--
			redraw(sh, before, line, pos)
			continue
		case 0x1b:
			params, final = readEscape(sh, buf)

			switch final {
			case 'Z':
				write("\n")
				toggleMode(sh)
				write("\r\x1b[2K%s%s", prompt(sh), string(line))
				histPos = len(historyFor(sh))
				draft = ""
			case 'u':
				if params == shiftEnterParams {
					write("\n")
					return string(line), errSoftNewline
				}
			case 'D':
				if params == ctrlArrowParams {
					pos = wordBoundaryLeft(line, pos)
					redraw(sh, string(line), line, pos)
				} else if pos > 0 {
					pos--
					redraw(sh, string(line), line, pos)
				}
			case 'C':
				if params == ctrlArrowParams {
					pos = wordBoundaryRight(line, pos)
					redraw(sh, string(line), line, pos)
				} else if pos < len(line) {
					pos++
					redraw(sh, string(line), line, pos)
				}
			case 'H':
				if pos > 0 {
					pos = 0
					redraw(sh, string(line), line, pos)
				}
			case 'F':
				if pos < len(line) {
					pos = len(line)
					redraw(sh, string(line), line, pos)
				}
			case '~':
				switch params {
				case homeKeyParams:
					if pos > 0 {
						pos = 0
						redraw(sh, string(line), line, pos)
					}
				case endKeyParams:
					if pos < len(line) {
						pos = len(line)
						redraw(sh, string(line), line, pos)
					}
				}
			case 'A':
				if histPos > 0 {
					if histPos == len(historyFor(sh)) {
						draft = string(line)
					}

					before = string(line)
					histPos--
					line = []rune(historyFor(sh)[histPos])
					pos = len(line)
					redraw(sh, before, line, pos)
				}
			case 'B':
				if histPos < len(historyFor(sh)) {
					before = string(line)
					histPos++

					if histPos == len(historyFor(sh)) {
						line = []rune(draft)
					} else {
						line = []rune(historyFor(sh)[histPos])
					}

					pos = len(line)
					redraw(sh, before, line, pos)
				}
			}

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

		if pos == len(line) {
			line = append(line, letter)
			pos++
			write("%s", string(letter))
		} else {
			before = string(line)
			line = append(line[:pos:pos], append([]rune{letter}, line[pos:]...)...)
			pos++
			redraw(sh, before, line, pos)
		}
	}
}
