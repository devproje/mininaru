// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

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
	if !strings.Contains(out, RED+"code"+RESET) {
		t.Error("inline code not styled as red text")
	}
	if strings.Contains(out, "\x1b[7m") {
		t.Error("inline code still using reverse-video")
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

const mdTableSample = "before\n\n| Name | Size | Note |\n|:-----|-----:|:----:|\n| a | 1 | x |\n| bb | 22 | yy |\n\nafter\n"

func TestMarkdownRendersTable(t *testing.T) {
	var out string
	var line string
	var header string
	var rule string
	var first string

	out = renderWhole(mdTableSample)

	if strings.Contains(out, "|") {
		t.Fatalf("table pipes leaked:\n%s", out)
	}

	for _, line = range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "Name") && strings.Contains(line, "Note"):
			header = line
		case strings.Contains(line, "─"):
			rule = line
		case strings.Contains(line, "bb"):
			first = line
		}
	}

	if header == "" || rule == "" || first == "" {
		t.Fatalf("missing header/rule/body row:\n%s", out)
	}

	if !strings.Contains(header, BOLD+"Name"+RESET) {
		t.Errorf("header cell not bold: %q", header)
	}

	if !strings.Contains(first, "  22") {
		t.Errorf("right-aligned Size column not padded on the left: %q", first)
	}

	if !strings.Contains(first, "bb  ") {
		t.Errorf("left-aligned Name column not padded on the right: %q", first)
	}
}

func TestMarkdownTableStreamingInvariant(t *testing.T) {
	var whole string
	var got string
	var i int

	whole = renderWhole(mdTableSample)

	for i = 0; i <= len(mdTableSample); i++ {
		got = renderSplit(mdTableSample, i)
		if got != whole {
			t.Fatalf("split at %d changed output:\n split: %q\n whole: %q", i, got, whole)
		}
	}
}

func TestMarkdownNonTablePipes(t *testing.T) {
	var out string

	out = renderWhole("| just some | text |\nnot a table\n")
	if !strings.Contains(out, "| just some | text |") {
		t.Fatalf("pipe line without a separator row should render literally: %q", out)
	}
}

func TestMarkdownKeepsMultibyteBytes(t *testing.T) {
	var md mdRenderer
	var got string

	got = md.write("안녕하세요, 잘돼요.\n")
	if got != "안녕하세요, 잘돼요.\n" {
		t.Fatalf("multibyte text mangled: %q", got)
	}

	got = md.write("**굵게**") + md.flush()
	if !strings.Contains(got, "굵게") {
		t.Fatalf("inline multibyte mangled: %q", got)
	}
}
