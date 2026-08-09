// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

var progressOut io.Writer = os.Stderr

var progressFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const progressInterval = 90 * time.Millisecond

type progress struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func progressTerminal() bool {
	var out *os.File
	var ok bool

	out, ok = progressOut.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(out.Fd()))
}

func progressAnimate(ctx context.Context, label string) {
	var ticker *time.Ticker
	var frame int

	ticker = time.NewTicker(progressInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprint(progressOut, "\r\033[K")

			return
		case <-ticker.C:
			fmt.Fprintf(progressOut, "\r\033[K%s %s", statusStyle.Render(progressFrames[frame]), hintStyle.Render(label))
			frame = (frame + 1) % len(progressFrames)
		}
	}
}

func progressStart(ctx context.Context, label string) *progress {
	var running *progress
	var animation context.Context

	running = &progress{done: make(chan struct{})}

	if !progressTerminal() {
		fmt.Fprintln(progressOut, hintStyle.Render(label+"…"))
		close(running.done)

		return running
	}

	animation, running.cancel = context.WithCancel(ctx)

	go func() {
		progressAnimate(animation, label)
		close(running.done)
	}()

	return running
}

func (p *progress) stop() {
	p.once.Do(func() {
		if p.cancel != nil {
			p.cancel()
		}

		<-p.done
	})
}

func withProgress(ctx context.Context, label string, run func() error) error {
	var running *progress

	var err error

	running = progressStart(ctx, label)

	err = run()

	running.stop()

	return err
}
