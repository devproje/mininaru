// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestReadLineInterjectsFrames(t *testing.T) {
	var e editor
	var frames chan Reply
	var stream keys
	var read, wr, stdout *os.File
	var captured []byte
	var line string

	var err error

	frames = make(chan Reply, 8)
	stream = make(keys, 8)

	frames <- Reply{Type: "message", Name: "coder", Message: "status?"}
	frames <- Reply{Type: "done"}

	read, wr, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout = os.Stdout
	os.Stdout = wr

	e = editor{keys: stream, frames: frames, prompt: "> ", onFrame: func(reply Reply) string {
		if reply.Type != "done" {
			return ""
		}

		stream <- 'h'
		stream <- 'i'
		stream <- '\r'

		return "AMBIENT\n"
	}}

	line, err = e.readLine()
	wr.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatal(err)
	}

	captured, _ = io.ReadAll(read)

	if line != "hi" {
		t.Fatalf("line = %q", line)
	}

	if !strings.Contains(string(captured), "AMBIENT") {
		t.Fatalf("interjected block not printed: %q", captured)
	}
}

func TestReadLineReturnsGoneOnClosedFrames(t *testing.T) {
	var e editor
	var frames chan Reply

	var err error

	frames = make(chan Reply)
	close(frames)

	e = editor{keys: make(keys), frames: frames, prompt: "> ", onFrame: func(Reply) string { return "" }}

	_, err = e.readLine()
	if !errors.Is(err, errGone) {
		t.Fatalf("want errGone, got %v", err)
	}
}

func TestCsiUCode(t *testing.T) {
	var code int
	var mods int

	code, mods = csiUCode("13;2")
	if code != 13 || mods != 2 {
		t.Fatalf("shift+enter: got %d;%d", code, mods)
	}

	code, mods = csiUCode("99;5")
	if code != 'c' || !modHas(mods, 0x04) {
		t.Fatalf("ctrl+c: got %d;%d ctrl=%v", code, mods, modHas(mods, 0x04))
	}

	code, mods = csiUCode("27")
	if code != 27 || mods != 1 {
		t.Fatalf("bare esc: got %d;%d", code, mods)
	}

	code, mods = csiUCode("97;1:2")
	if code != 97 || mods != 1 {
		t.Fatalf("event-type suffix: got %d;%d", code, mods)
	}
}
