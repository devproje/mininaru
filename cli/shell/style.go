// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/devproje/mininaru/util"
)

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

	BASH_BADGE  string = "\x1b[48;5;110m\x1b[38;5;235m bash  \x1b[0m\x1b[38;5;110m"
	AGENT_BADGE string = "\x1b[48;5;141m\x1b[38;5;235m agent \x1b[0m\x1b[38;5;141m"
	USER_BG     string = "\x1b[48;5;245m\x1b[38;5;235m"
	ROOT_BG     string = "\x1b[48;5;203m\x1b[38;5;231m\x1b[1m"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

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

func write(format string, args ...any) {
	fmt.Print(strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", "\r\n"))
}

func notice(color string, mark string, format string, args ...any) {
	write("%s%s%s %s\n", color, mark, RESET, fmt.Sprintf(format, args...))
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

		tick = time.NewTicker(SPINNER_TICK)
		defer tick.Stop()
		defer close(done)

		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				write("\r\x1b[2K%s%s%s %s%s%s", PURPLE, spinnerFrames[i%len(spinnerFrames)], RESET, DIM, label, RESET)
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

func currentUser() string {
	var current *user.User
	var name string

	var err error

	current, err = user.Current()
	if err == nil && current.Username != "" {
		return current.Username
	}

	name = os.Getenv("USER")
	if name != "" {
		return name
	}

	return fmt.Sprintf("uid:%d", os.Geteuid())
}

func agentLabel(sh *state) string {
	if sh.name != "" {
		return sh.name
	}

	if sh.agent != "" {
		return sh.agent
	}

	return "agent"
}

func userBadge(sh *state) string {
	if sh.root {
		return fmt.Sprintf("%s %s %s", ROOT_BG, sh.user, RESET)
	}

	return fmt.Sprintf("%s %s %s", USER_BG, sh.user, RESET)
}

func prompt(sh *state) string {
	var badge string
	var caret string

	badge = BASH_BADGE
	caret = BLUE + "$" + RESET

	if sh.mode == MODE_AGENT {
		badge = AGENT_BADGE
		caret = PURPLE + "❯" + RESET
	}

	if sh.root {
		caret = RED + "#" + RESET
	}

	return fmt.Sprintf("%s%s%s %s%s%s %s ", userBadge(sh), badge, RESET, DIM, shortPath(sh.cwd), RESET, caret)
}

func banner(sh *state) {
	write("\n%s\n\n", util.NaruLogoWithPad("  "))
	write("  %smininaru shell%s %s%s%s\n", BOLD, RESET, DIM, util.AppVersion, RESET)
	write("  %sShift+Tab%s switch mode   %sCtrl+D%s exit   %sCtrl+U%s clear line\n\n", GRAY, RESET, GRAY, RESET, GRAY, RESET)

	if sh.conn != nil {
		notice(GREEN, "●", "%sconnected%s %s", GREEN, RESET, DIM+sh.url+RESET)
		return
	}

	notice(YELLOW, "○", "%soffline%s %s", YELLOW, RESET, DIM+"bash mode only, Shift+Tab retries the connection"+RESET)
}
