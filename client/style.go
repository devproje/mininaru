// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
	var out string
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

		out = fmt.Sprintf("%s%c", out, runes[i])
		i++
	}

	return out
}

const (
	RESET string = "\x1b[0m"
	DIM   string = "\x1b[2m"
	BOLD  string = "\x1b[1m"

	RED    string = "\x1b[38;5;203m"
	GREEN  string = "\x1b[38;5;114m"
	YELLOW string = "\x1b[38;5;179m"
	BLUE   string = "\x1b[38;5;110m"
	PURPLE string = "\x1b[38;5;141m"
	GRAY   string = "\x1b[38;5;245m"
)

const (
	spinnerTick  time.Duration = 80 * time.Millisecond
	barWidth     int           = 10
	barSegment   int           = 3
	barStepTicks int           = 2
)

func write(format string, args ...any) {
	fmt.Print(strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", "\r\n"))
}

func notice(color string, mark string, format string, args ...any) {
	write("%s%s%s %s\n", color, mark, RESET, fmt.Sprintf(format, args...))
}

func displayWidth(text string) int {
	var letter rune
	var width int

	for _, letter = range text {
		switch {
		case letter == 0:
		case letter < 0x20:
		case letter >= 0x1100 && (letter <= 0x115f ||
			letter == 0x2329 || letter == 0x232a ||
			(letter >= 0x2e80 && letter <= 0xa4cf && letter != 0x303f) ||
			(letter >= 0xac00 && letter <= 0xd7a3) ||
			(letter >= 0xf900 && letter <= 0xfaff) ||
			(letter >= 0xfe30 && letter <= 0xfe6f) ||
			(letter >= 0xff00 && letter <= 0xff60) ||
			(letter >= 0xffe0 && letter <= 0xffe6) ||
			(letter >= 0x1f300 && letter <= 0x1f64f) ||
			(letter >= 0x20000 && letter <= 0x3fffd)):
			width = width + 2
		default:
			width = width + 1
		}
	}

	return width
}

func barFrame(tick int) string {
	var span int
	var pos int

	span = barWidth - barSegment + 1
	pos = (tick / barStepTicks) % span

	return fmt.Sprintf("%s%s%s",
		strings.Repeat("░", pos),
		strings.Repeat("█", barSegment),
		strings.Repeat("░", barWidth-pos-barSegment))
}

func spinner(label string) func() {
	var stop chan struct{}
	var done chan struct{}
	var once sync.Once

	stop = make(chan struct{})
	done = make(chan struct{})

	go func() {
		var tick *time.Ticker
		var i int

		tick = time.NewTicker(spinnerTick)
		defer tick.Stop()
		defer close(done)

		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				write("\r\x1b[2K%s%s%s %s%s%s", PURPLE, barFrame(i), RESET, DIM, label, RESET)
				i++
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stop)
			<-done
			write("\r\x1b[2K")
		})
	}
}

func shortPath(cwd string) string {
	var home string
	var parts []string

	var err error

	home, err = os.UserHomeDir()
	if err == nil && strings.HasPrefix(cwd, home) {
		cwd = "~" + strings.TrimPrefix(cwd, home)
	}

	parts = strings.Split(cwd, string(os.PathSeparator))
	if len(parts) <= 3 {
		return cwd
	}

	return strings.Join(append([]string{parts[0], "…"}, parts[len(parts)-2:]...), string(os.PathSeparator))
}

func pathColor(mode string) string {
	switch mode {
	case "persist":
		return YELLOW
	case "on":
		return RED
	}

	return DIM
}

func effortColor(level string) string {
	switch level {
	case "off":
		return DIM
	case "low":
		return BLUE
	case "high":
		return YELLOW
	case "max":
		return RED
	}

	return GRAY
}
