// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package shell

import (
	"os"
	"testing"
	"time"
)

func fakeStdin(t *testing.T) *os.File {
	var reader *os.File
	var writer *os.File
	var previous *os.File

	var err error

	reader, writer, err = os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	previous = os.Stdin
	os.Stdin = reader

	t.Cleanup(func() {
		os.Stdin = previous
		reader.Close()
		writer.Close()
	})

	return writer
}

func waitInterrupted(t *testing.T, w *interruptWatch) {
	select {
	case <-w.interrupted:
	case <-time.After(2 * time.Second):
		t.Fatal("interrupt was not detected within 2s")
	}
}

func TestInterruptWatchFiresOnEscape(t *testing.T) {
	var stdin *os.File
	var watch *interruptWatch

	stdin = fakeStdin(t)
	watch = newInterruptWatch()
	defer watch.pause()

	stdin.Write([]byte{0x1b})

	waitInterrupted(t, watch)
}

func TestInterruptWatchResumesAfterPause(t *testing.T) {
	var stdin *os.File
	var watch *interruptWatch

	stdin = fakeStdin(t)
	watch = newInterruptWatch()
	defer watch.pause()

	watch.pause()

	stdin.Write([]byte{0x03})
	time.Sleep(200 * time.Millisecond)

	select {
	case <-watch.interrupted:
		t.Fatal("a paused watch must not fire")
	default:
	}

	watch.resume()

	stdin.Write([]byte{0x03})
	waitInterrupted(t, watch)
}

func TestInterruptWatchCapturesTypedBytes(t *testing.T) {
	var stdin *os.File
	var watch *interruptWatch
	var deadline time.Time

	stdin = fakeStdin(t)
	watch = newInterruptWatch()

	stdin.Write([]byte("ls"))

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if string(watch.capturedInput()) == "ls" {
			break
		}

		time.Sleep(20 * time.Millisecond)
	}

	watch.pause()

	if string(watch.capturedInput()) != "ls" {
		t.Fatalf("capturedInput() = %q, want \"ls\"", watch.capturedInput())
	}
}

func TestInterruptWatchPauseIsIdempotent(t *testing.T) {
	var watch *interruptWatch

	fakeStdin(t)
	watch = newInterruptWatch()

	watch.pause()
	watch.pause()
}
