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

func runSelector(t *testing.T, items []string, bytesIn []byte) (int, error, string) {
	var stream keys
	var read, wr, stdout *os.File
	var captured []byte
	var pick int
	var b byte

	var err error

	stream = make(keys, 64)
	for _, b = range bytesIn {
		stream <- b
	}

	read, wr, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout = os.Stdout
	os.Stdout = wr

	pick, err = selectFrom("pick", items, stream)

	wr.Close()
	os.Stdout = stdout

	captured, _ = io.ReadAll(read)

	return pick, err, string(captured)
}

func TestSelectorMovesAndPicks(t *testing.T) {
	var pick int
	var out string

	var err error

	pick, err, out = runSelector(t, []string{"a", "b", "c"}, []byte{0x1b, '[', 'B', 0x1b, '[', 'B', '\r'})
	if err != nil {
		t.Fatal(err)
	}

	if pick != 2 {
		t.Fatalf("down down enter -> want 2, got %d", pick)
	}

	if !strings.Contains(out, "pick") || !strings.Contains(out, "a") {
		t.Fatalf("menu not rendered: %q", out)
	}
}

func TestSelectorClampsAndUsesVim(t *testing.T) {
	var pick int

	var err error

	pick, err, _ = runSelector(t, []string{"a", "b", "c"}, []byte{'k', 'k', 'j', '\r'})
	if err != nil {
		t.Fatal(err)
	}

	if pick != 1 {
		t.Fatalf("k k j from 0 -> want 1, got %d", pick)
	}
}

func TestSelectorEscapeCancels(t *testing.T) {
	var pick int

	var err error

	pick, err, _ = runSelector(t, []string{"a", "b"}, []byte{0x1b, 'z'})
	if !errors.Is(err, errInterrupted) {
		t.Fatalf("bare esc -> want errInterrupted, got %v", err)
	}

	if pick != -1 {
		t.Fatalf("cancel -> want -1, got %d", pick)
	}
}
