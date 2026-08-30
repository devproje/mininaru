// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"fmt"
	"strings"
)

type ambient struct {
	name    string
	ask     string
	reply   string
	failure string
	tools   []string
}

func (a *ambient) feed(reply Reply) {
	switch reply.Type {
	case "message":
		a.name = reply.Name
		a.ask = reply.Message
	case "chunk":
		if reply.Chunk != nil && len(reply.Chunk.Choices) > 0 {
			a.reply = a.reply + reply.Chunk.Choices[0].Delta.Content
		}
	case "tool":
		if reply.Status == "finished" || reply.Status == "failed" {
			a.tools = append(a.tools, reply.Name)
		}
	case "error":
		a.failure = reply.Message
	}
}

func (a *ambient) gutter(color string, text string) string {
	var line string
	var out strings.Builder

	for _, line = range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		out.WriteString(fmt.Sprintf("%s┆%s %s\n", BLUE, RESET, color+line+RESET))
	}

	return out.String()
}

func (a *ambient) flush() string {
	var head string
	var out strings.Builder

	head = a.name
	if head == "" {
		head = "another session"
	}

	out.WriteString("\n")
	out.WriteString(fmt.Sprintf("%s┆%s %s⇄ %s%s\n", BLUE, RESET, BOLD, head, RESET))

	if a.ask != "" {
		out.WriteString(a.gutter(DIM, a.ask))
	}

	if a.reply != "" {
		out.WriteString(a.gutter("", a.reply))
	}

	if a.failure != "" {
		out.WriteString(a.gutter(RED, "✗ "+a.failure))
	}

	if len(a.tools) > 0 {
		out.WriteString(a.gutter(DIM, "· "+strings.Join(a.tools, ", ")))
	}

	out.WriteString("\n")

	*a = ambient{}

	return out.String()
}
