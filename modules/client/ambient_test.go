// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"strings"
	"testing"
)

func TestAmbientBuildsBlockOnce(t *testing.T) {
	var box ambient
	var block string
	var again string

	box.feed(Reply{Type: "message", Name: "coder", Message: "build status?"})
	box.feed(Reply{Type: "chunk", Chunk: chunkContent("green ")})
	box.feed(Reply{Type: "chunk", Chunk: chunkContent("across the board")})
	box.feed(Reply{Type: "tool", Name: "bash", Status: "finished"})

	block = box.flush()

	for _, want := range []string{"coder", "build status?", "green across the board", "bash", "┆"} {
		if !strings.Contains(block, want) {
			t.Fatalf("block missing %q:\n%s", want, block)
		}
	}

	again = box.flush()
	if strings.Contains(again, "coder") || strings.Contains(again, "green") {
		t.Fatalf("flush did not reset: %q", again)
	}
}

func TestAmbientCarriesFailure(t *testing.T) {
	var box ambient
	var block string

	box.feed(Reply{Type: "message", Name: "coder", Message: "deploy"})
	box.feed(Reply{Type: "error", Message: "boom"})

	block = box.flush()
	if !strings.Contains(block, "boom") {
		t.Fatalf("failure not shown: %q", block)
	}
}
