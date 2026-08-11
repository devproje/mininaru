// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
)

const stdinPrompt = "-"

func promptContent(value string, in io.Reader) (string, error) {
	var buf []byte

	var err error

	if value != stdinPrompt {
		return value, nil
	}

	buf, err = io.ReadAll(in)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(buf)), nil
}

func promptToolLog(logs io.Writer, event core.ToolEvent) {
	var label string

	label = core.ToolLabel(event.Name, event.Arguments)

	if event.Phase == core.ToolEventStarted {
		fmt.Fprintf(logs, "tool %s started\n", label)
		return
	}

	if event.Error != "" {
		fmt.Fprintf(logs, "tool %s failed: %s\n", label, event.Error)
		return
	}

	fmt.Fprintf(logs, "tool %s completed\n", label)
}

func runPrompt(ctx context.Context, out, logs io.Writer, session *core.Session, agent *core.NaruAgent, content string) error {
	var message *core.Message
	var waiting *progress

	var err error

	waiting = progressStart(ctx, "thinking")

	message, err = core.ChatWithApproval(ctx, session, agent, content, nil,
		func(delta string) {
			waiting.stop()

			if !config.Client.Thinking.Show {
				return
			}

			io.WriteString(logs, delta)
		},
		func(event core.ToolEvent) {
			waiting.stop()

			promptToolLog(logs, event)
		}, nil)

	waiting.stop()

	if err != nil {
		return err
	}

	fmt.Fprintln(out, message.Content)

	return nil
}
