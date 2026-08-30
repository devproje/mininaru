// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	ctrlArrowParams string = "1;5"
	homeKeyParams   string = "1"
	endKeyParams    string = "4"
	enterKey        int    = 13
	kittyKbEnable   string = "\x1b[>1u"
	kittyKbDisable  string = "\x1b[<u"
)

var errInterrupted error = errors.New("interrupted")

type keys chan byte

func newKeys() keys {
	var stream keys

	stream = make(keys, 256)

	go func() {
		var buf []byte
		var count int

		var err error

		buf = make([]byte, 1)

		for {
			count, err = os.Stdin.Read(buf)
			if err != nil {
				close(stream)

				return
			}

			if count == 0 {
				continue
			}

			stream <- buf[0]
		}
	}()

	return stream
}

func (k keys) next() (byte, error) {
	var b byte
	var ok bool

	b, ok = <-k
	if !ok {
		return 0, io.EOF
	}

	return b, nil
}

type editor struct {
	keys    keys
	history []string
	kill    []rune
	prompt  string
}

func (k keys) escape() (string, byte) {
	var params []byte
	var b byte

	var err error

	b, err = k.next()
	if err != nil || b != '[' {
		return "", 0
	}

	for {
		b, err = k.next()
		if err != nil {
			return "", 0
		}

		if b >= '@' && b <= '~' {
			return string(params), b
		}

		params = append(params, b)
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

func csiUCode(params string) (int, int) {
	var fields []string
	var code int
	var mods int

	var err error

	fields = strings.Split(params, ";")

	code, err = strconv.Atoi(strings.SplitN(fields[0], ":", 2)[0])
	if err != nil {
		return 0, 1
	}

	mods = 1
	if len(fields) > 1 {
		mods, err = strconv.Atoi(strings.SplitN(fields[1], ":", 2)[0])
		if err != nil || mods < 1 {
			mods = 1
		}
	}

	return code, mods
}

func modHas(mods int, bit int) bool {
	return (mods-1)&bit != 0
}

func (e *editor) redraw(before string, line []rune, pos int) {
	var cols int
	var oldRows int
	var trailing int

	cols = termWidth()
	oldRows = rowsFor(cols, e.prompt+before)

	if oldRows > 1 {
		write("\x1b[%dA", oldRows-1)
	}

	write("\r\x1b[0J%s%s", e.prompt, string(line))

	trailing = displayWidth(string(line[pos:]))
	if trailing > 0 {
		write("\x1b[%dD", trailing)
	}
}

func (e *editor) readLine() (string, error) {
	var buf []byte
	var synth byte
	var haveSynth bool
	var line []rune
	var pending []byte
	var before string
	var draft string
	var params string
	var letter rune
	var final byte
	var code int
	var mods int
	var pos int
	var killFrom int
	var histPos int

	var err error

	buf = make([]byte, 1)
	histPos = len(e.history)

	write("%s", e.prompt)

	for {
		if haveSynth {
			buf[0] = synth
			haveSynth = false
		} else {
			buf[0], err = e.keys.next()
			if err != nil {
				return "", err
			}
		}

		switch buf[0] {
		case 0x01:
			if pos > 0 {
				pos = 0
				e.redraw(string(line), line, pos)
			}

			continue
		case 0x03:
			write("%s^C%s\n", GRAY, RESET)

			return "", errInterrupted
		case 0x04:
			return "", io.EOF
		case 0x05:
			if pos < len(line) {
				pos = len(line)
				e.redraw(string(line), line, pos)
			}

			continue
		case 0x0b:
			if pos == len(line) {
				continue
			}

			e.kill = append([]rune{}, line[pos:]...)
			before = string(line)
			line = line[:pos]
			e.redraw(before, line, pos)

			continue
		case 0x0c:
			write("\x1b[H\x1b[2J")
			e.redraw("", line, pos)

			continue
		case 0x15:
			if pos == 0 {
				continue
			}

			e.kill = append([]rune{}, line[:pos]...)
			before = string(line)
			line = append([]rune{}, line[pos:]...)
			pos = 0
			e.redraw(before, line, pos)

			continue
		case 0x17:
			if pos == 0 {
				continue
			}

			killFrom = wordBoundaryLeft(line, pos)
			e.kill = append([]rune{}, line[killFrom:pos]...)
			before = string(line)
			line = append(line[:killFrom:killFrom], line[pos:]...)
			pos = killFrom
			e.redraw(before, line, pos)

			continue
		case 0x19:
			if len(e.kill) == 0 {
				continue
			}

			before = string(line)
			line = append(line[:pos:pos], append(append([]rune{}, e.kill...), line[pos:]...)...)
			pos = pos + len(e.kill)
			e.redraw(before, line, pos)

			continue
		case '\r':
			write("\n")

			return string(line), nil
		case '\n':
			line = append(line, '\n')
			pos = len(line)
			write("\n")

			continue
		case 0x7f, 0x08:
			if pos == 0 {
				continue
			}

			before = string(line)
			line = append(line[:pos-1], line[pos:]...)
			pos--
			e.redraw(before, line, pos)

			continue
		case 0x1b:
			params, final = e.keys.escape()

			switch final {
			case 'u':
				code, mods = csiUCode(params)

				if code == enterKey {
					line = append(line, '\n')
					pos = len(line)
					write("\n")
					continue
				}

				if modHas(mods, 0x04) && code >= 'a' && code <= 'z' {
					synth = byte(code) - 0x60
					haveSynth = true
				}
			case 'D':
				if params == ctrlArrowParams {
					pos = wordBoundaryLeft(line, pos)
					e.redraw(string(line), line, pos)
					continue
				}

				if pos > 0 {
					pos--
					e.redraw(string(line), line, pos)
				}
			case 'C':
				if params == ctrlArrowParams {
					pos = wordBoundaryRight(line, pos)
					e.redraw(string(line), line, pos)
					continue
				}

				if pos < len(line) {
					pos++
					e.redraw(string(line), line, pos)
				}
			case 'H':
				if pos > 0 {
					pos = 0
					e.redraw(string(line), line, pos)
				}
			case 'F':
				if pos < len(line) {
					pos = len(line)
					e.redraw(string(line), line, pos)
				}
			case '~':
				switch params {
				case homeKeyParams:
					if pos > 0 {
						pos = 0
						e.redraw(string(line), line, pos)
					}
				case endKeyParams:
					if pos < len(line) {
						pos = len(line)
						e.redraw(string(line), line, pos)
					}
				}
			case 'A':
				if histPos == 0 {
					continue
				}

				if histPos == len(e.history) {
					draft = string(line)
				}

				before = string(line)
				histPos--
				line = []rune(e.history[histPos])
				pos = len(line)
				e.redraw(before, line, pos)
			case 'B':
				if histPos >= len(e.history) {
					continue
				}

				before = string(line)
				histPos++
				line = []rune(draft)

				if histPos < len(e.history) {
					line = []rune(e.history[histPos])
				}

				pos = len(line)
				e.redraw(before, line, pos)
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

			continue
		}

		before = string(line)
		line = append(line[:pos:pos], append([]rune{letter}, line[pos:]...)...)
		pos++
		e.redraw(before, line, pos)
	}
}
