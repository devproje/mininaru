// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

type session struct {
	ctx      context.Context
	cancel   context.CancelFunc
	lastUsed time.Time
}

const idleTimeout = 5 * time.Minute
const reaperInterval = 1 * time.Minute

var chromeCandidates = []string{
	"headless-shell", "headless_shell", "chromium-headless-shell",
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome",
}

var chromeAbsolutePaths = []string{
	"/usr/lib64/chromium-browser/headless_shell",
	"/usr/lib/chromium-browser/headless_shell",
	"/usr/lib64/chromium/headless_shell",
	"/usr/lib/chromium/headless_shell",
}

var mu sync.Mutex
var sessions = make(map[string]*session)
var reaperOnce sync.Once

func chromePath() string {
	var path string
	var candidate string
	var resolved string
	var info os.FileInfo

	var err error

	path = os.Getenv("MININARU_CHROME")
	if path != "" {
		return path
	}

	for _, candidate = range chromeCandidates {
		resolved, err = exec.LookPath(candidate)
		if err == nil {
			return resolved
		}
	}

	for _, candidate = range chromeAbsolutePaths {
		info, err = os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}

	return ""
}

func Available() bool {
	return chromePath() != ""
}

func allocatorOptions() []chromedp.ExecAllocatorOption {
	var opts []chromedp.ExecAllocatorOption
	var path string

	opts = append(opts, chromedp.DefaultExecAllocatorOptions[:]...)

	path = chromePath()
	if path != "" {
		opts = append(opts, chromedp.ExecPath(path))
	}

	return opts
}

func newSession() (*session, error) {
	var allocCtx context.Context
	var allocCancel context.CancelFunc
	var ctx context.Context
	var cancel context.CancelFunc
	var current session
	var launched chan error
	var err error

	allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), allocatorOptions()...)
	ctx, cancel = chromedp.NewContext(allocCtx)

	current.ctx = ctx
	current.lastUsed = time.Now()
	current.cancel = func() {
		cancel()
		allocCancel()
	}

	launched = make(chan error, 1)
	go func() {
		launched <- chromedp.Run(ctx)
	}()

	select {
	case err = <-launched:
		if err != nil {
			current.cancel()
			return nil, err
		}
	case <-time.After(callTimeout):
		current.cancel()
		return nil, fmt.Errorf("timed out launching the browser after %s", callTimeout)
	}

	return &current, nil
}

func startReaper() {
	go func() {
		var tick *time.Ticker
		var now time.Time
		var id string
		var current *session
		var stale []string

		tick = time.NewTicker(reaperInterval)
		defer tick.Stop()

		for range tick.C {
			now = time.Now()
			stale = nil

			mu.Lock()
			for id, current = range sessions {
				if now.Sub(current.lastUsed) > idleTimeout {
					stale = append(stale, id)
				}
			}
			for _, id = range stale {
				sessions[id].cancel()
				delete(sessions, id)
			}
			mu.Unlock()
		}
	}()
}

func sessionContext(sessionId string) (context.Context, error) {
	var current *session
	var ok bool

	var err error

	reaperOnce.Do(startReaper)

	mu.Lock()
	defer mu.Unlock()

	current, ok = sessions[sessionId]
	if !ok {
		current, err = newSession()
		if err != nil {
			return nil, err
		}

		sessions[sessionId] = current
	}

	current.lastUsed = time.Now()

	return current.ctx, nil
}

func withCallTimeout(sessionId string) (context.Context, context.CancelFunc, error) {
	var parent context.Context
	var ctx context.Context
	var cancel context.CancelFunc

	var err error

	parent, err = sessionContext(sessionId)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel = context.WithTimeout(parent, callTimeout)

	return ctx, cancel, nil
}

func closeSession(sessionId string) {
	var current *session
	var ok bool

	mu.Lock()
	defer mu.Unlock()

	current, ok = sessions[sessionId]
	if !ok {
		return
	}

	current.cancel()
	delete(sessions, sessionId)
}
