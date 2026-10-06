// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package client

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const mdSample = "# Title\n\nsome **bold** and `code` and *em*.\n\n- one\n- two\n1. first\n\n> quoted\n\n```go\nfmt.Println(\"x\")\n```\n\ntail without newline"

func renderWhole(md string) string {
	var r MdRenderer
	return r.Write(md) + r.Flush()
}

func renderSplit(md string, at int) string {
	var r MdRenderer
	return r.Write(md[:at]) + r.Write(md[at:]) + r.Flush()
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
	if !strings.Contains(ansi.Strip(out), "│ fmt.Println(\"x\")") {
		t.Error("fenced code line missing gutter or syntax highlighting broke content")
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
	var md MdRenderer
	var got string

	got = md.Write("안녕하세요, 잘돼요.\n")
	if got != "안녕하세요, 잘돼요.\n" {
		t.Fatalf("multibyte text mangled: %q", got)
	}

	got = md.Write("**굵게**") + md.Flush()
	if !strings.Contains(got, "굵게") {
		t.Fatalf("inline multibyte mangled: %q", got)
	}
}

func TestMarkdownRendersStrikethrough(t *testing.T) {
	var out string

	out = renderWhole("~~gone~~ stays")

	if !strings.Contains(out, "\x1b[9mgone\x1b[29m") {
		t.Fatalf("strikethrough not applied: %q", out)
	}
	if strings.Contains(out, "~~") {
		t.Fatalf("tilde markers leaked into output: %q", out)
	}
}
