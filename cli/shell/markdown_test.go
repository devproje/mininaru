// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"strings"
	"testing"
)

const mdSample = "# Title\n\nsome **bold** and `code` and *em*.\n\n- one\n- two\n1. first\n\n> quoted\n\n```go\nfmt.Println(\"x\")\n```\n\ntail without newline"

func renderWhole(md string) string {
	var r mdRenderer
	return r.write(md) + r.flush()
}

func renderSplit(md string, at int) string {
	var r mdRenderer
	return r.write(md[:at]) + r.write(md[at:]) + r.flush()
}

func TestMarkdownStreamingInvariant(t *testing.T) {
	var whole string
	var got string
	var i int

	whole = renderWhole(mdSample)

	for i = 0; i <= len(mdSample); i++ {
		got = renderSplit(mdSample, i)
		if got != whole {
			t.Fatalf("split at %d changed output:\n split: %q\n whole: %q", i, got, whole)
		}
	}
}

func TestMarkdownRendersElements(t *testing.T) {
	var out string

	out = renderWhole(mdSample)

	if strings.Contains(out, "# Title") {
		t.Error("heading hashes not stripped")
	}
	if !strings.Contains(out, BOLD+PURPLE+"Title"+RESET) {
		t.Error("heading not styled")
	}
	if !strings.Contains(out, BOLD+"bold"+RESET) {
		t.Error("**bold** not styled")
	}
	if !strings.Contains(out, "• "+RESET+"one") {
		t.Error("bullet list marker not normalised")
	}
	if !strings.Contains(out, "• "+RESET+"first") {
		t.Error("ordered list marker not normalised")
	}
	if !strings.Contains(out, "│ "+RESET+"fmt.Println") {
		t.Error("fenced code line missing gutter / got inline-processed")
	}
	if strings.Contains(out, "```") {
		t.Error("fence markers leaked into output")
	}
	if !strings.Contains(out, "tail without newline") {
		t.Error("flush() dropped the trailing partial line")
	}
}

func TestMarkdownFenceSuppressesInline(t *testing.T) {
	var out string

	out = renderWhole("```\n**not bold** `not code`\n```\n")
	if !strings.Contains(out, "**not bold** `not code`") {
		t.Errorf("inline markup was processed inside a fence: %q", out)
	}
}
