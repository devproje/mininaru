// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package client

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devproje/mininaru/core"
)

const (
	RESET string = "\x1b[0m"
	DIM   string = "\x1b[2m"
	BOLD  string = "\x1b[1m"

	RED    string = "\x1b[38;5;203m"
	GREEN  string = "\x1b[38;5;114m"
	YELLOW string = "\x1b[38;5;179m"
	GOLD   string = "\x1b[38;5;178m"
	BLUE   string = "\x1b[38;5;110m"
	PURPLE string = "\x1b[38;5;141m"
	CYAN   string = "\x1b[38;5;80m"
	GRAY   string = "\x1b[38;5;245m"
	WHITE  string = "\x1b[38;5;255m"
)

func ModeColor(mode string) string {
	switch mode {
	case core.ModePlan:
		return GREEN
	case core.ModeAutoPersist:
		return GOLD
	case core.ModeFullAuto:
		return RED
	default:
		return GRAY
	}
}

func ModeColorCode(mode string) string {
	switch mode {
	case core.ModePlan:
		return "114"
	case core.ModeAutoPersist:
		return "178"
	case core.ModeFullAuto:
		return "203"
	default:
		return "245"
	}
}

const (
	SpinnerTick  time.Duration = 80 * time.Millisecond
	barWidth     int           = 10
	barSegment   int           = 3
	barStepTicks int           = 2
)

func BarFrame(tick int) string {
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

		tick = time.NewTicker(SpinnerTick)
		defer tick.Stop()
		defer close(done)

		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				write("\r\x1b[2K%s%s%s %s%s%s", PURPLE, BarFrame(i), RESET, WHITE, label, RESET)
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

func write(format string, args ...any) {
	fmt.Print(strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", "\r\n"))
}

func ContextLabel(usage *core.ContextUsage) string {
	var percent uint64
	var label string
	var cachePercent uint64

	if usage == nil || usage.Limit == 0 {
		return ""
	}

	percent = usage.Used * 100 / usage.Limit
	label = fmt.Sprintf("ctx:%d/%d (%d%%)", usage.Used, usage.Limit, percent)

	if usage.Cached == 0 || usage.Used == 0 {
		return label
	}

	cachePercent = usage.Cached * 100 / usage.Used

	return fmt.Sprintf("%s %scache:%d%%%s", label, CYAN, cachePercent, RESET)
}
