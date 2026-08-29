// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"fmt"
	"os"
	"strings"

	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

type renderer struct {
	conn     *websocket.Conn
	session  string
	keys     keys
	md       mdRenderer
	mode     string
	rich     bool
	stop     func()
	mu       sync.Mutex
	awaiting atomic.Bool
	answers  chan byte
}

func (r *renderer) send(frame Frame) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.conn.WriteJSON(frame)
}

func (r *renderer) watch(done chan struct{}) {
	var b byte
	var ok bool

	for {
		select {
		case <-done:
			return
		case b, ok = <-r.keys:
			if !ok {
				return
			}

			if r.awaiting.Load() {
				r.answers <- b

				continue
			}

			if b != 0x03 && b != 0x1b {
				continue
			}

			r.endLine()
			write("%s^C interrupting…%s\n", GRAY, RESET)
			r.send(Frame{Type: "interrupt", SessionId: r.session})
		}
	}
}

func (r *renderer) key() string {
	var b byte

	if r.keys == nil {
		return readKey()
	}

	r.awaiting.Store(true)
	defer r.awaiting.Store(false)

	b = <-r.answers

	return strings.ToLower(string(b))
}

func isTty() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func readKey() string {
	var state *term.State
	var buf []byte
	var fd int

	var err error

	fd = int(os.Stdin.Fd())
	buf = make([]byte, 1)

	if !term.IsTerminal(fd) {
		_, err = os.Stdin.Read(buf)
		if err != nil {
			return ""
		}

		return strings.ToLower(strings.TrimSpace(string(buf)))
	}

	state, err = term.MakeRaw(fd)
	if err != nil {
		return ""
	}
	defer term.Restore(fd, state)

	_, err = os.Stdin.Read(buf)
	if err != nil {
		return ""
	}

	return strings.ToLower(string(buf))
}

func (r *renderer) decide(name string, arguments string) string {
	write("\n%s%s%s wants to run %s\n", PURPLE, name, RESET, strings.TrimSpace(arguments))
	write("allow? %s[y]es / [a]ll / [N]o%s: ", GRAY, RESET)

	switch r.key() {
	case "y":
		write("y\n")

		return "once"
	case "a":
		write("a\n")

		return "session"
	}

	write("N\n")

	return "deny"
}

func (r *renderer) halt() {
	if r.stop == nil {
		return
	}

	r.stop()
	r.stop = nil
}

func (r *renderer) endLine() {
	r.halt()

	if r.mode == "" {
		return
	}

	if r.rich && r.mode == "content" {
		write("%s", r.md.flush())
	}

	write("%s\n", RESET)
	r.mode = ""
}

func (r *renderer) text(next string, text string) {
	if text == "" {
		return
	}

	r.halt()

	if next != r.mode {
		r.endLine()

		if next == "reasoning" {
			write("%s⋮ thinking%s\n%s", GRAY, RESET, DIM)
		}

		r.mode = next
	}

	if r.rich && next == "content" {
		write("%s", r.md.write(text))

		return
	}

	if next == "reasoning" {
		write("%s", strings.ReplaceAll(text, "\n", "\n"+DIM))

		return
	}

	write("%s", text)
}

func (r *renderer) tool(name string, status string, message string) {
	if status == "started" {
		r.endLine()
		r.stop = spinner(fmt.Sprintf("%s %s", name, message))

		return
	}

	r.endLine()
	write("%s· %s %s %s%s\n", DIM, name, status, message, RESET)
}

func (r *renderer) frame(reply Reply) (bool, error) {
	var err error

	switch reply.Type {
	case "chunk":
		if reply.Reasoning != "" {
			r.text("reasoning", reply.Reasoning)

			return false, nil
		}

		if reply.Chunk != nil && len(reply.Chunk.Choices) > 0 {
			r.text("content", reply.Chunk.Choices[0].Delta.Content)
		}
	case "message":
		r.endLine()
		write("%s← %s%s %s\n", GRAY, reply.Name, RESET, reply.Message)
	case "tool":
		r.tool(reply.Name, reply.Status, reply.Message)
	case "approval_request":
		r.endLine()

		err = r.send(Frame{
			Type:      "approval",
			SessionId: r.session,
			Decision:  r.decide(reply.Name, reply.Arguments),
		})
		if err != nil {
			return true, err
		}
	case "error":
		r.endLine()

		if strings.Contains(reply.Message, "context canceled") {
			write("%sinterrupted%s\n", GRAY, RESET)

			return true, nil
		}

		return true, fmt.Errorf("%s", reply.Message)
	case "done":
		r.endLine()

		return true, nil
	}

	return false, nil
}

func Receive(conn *websocket.Conn, session string, stream keys) error {
	var render *renderer
	var reply Reply
	var done chan struct{}
	var stop bool

	var err error

	render = &renderer{conn: conn, session: session, keys: stream, rich: isTty(), answers: make(chan byte, 1)}
	defer render.halt()

	if stream != nil {
		done = make(chan struct{})
		go render.watch(done)

		defer close(done)
	}

	for {
		reply = Reply{}

		err = conn.ReadJSON(&reply)
		if err != nil {
			return err
		}

		stop, err = render.frame(reply)
		if stop {
			return err
		}
	}
}
