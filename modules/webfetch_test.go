// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func renderHTML(t *testing.T, raw string) string {
	var document *html.Node

	var err error

	t.Helper()

	document, err = html.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	return htmlDocument(document)
}

func TestHTMLDocumentDropsScriptAndStyle(t *testing.T) {
	var rendered string

	rendered = renderHTML(t, `<html><head><style>body{color:red}</style></head>
		<body><script>var secret = 1;</script><p>First paragraph.</p><p>Second paragraph.</p></body></html>`)

	if strings.Contains(rendered, "secret") || strings.Contains(rendered, "color:red") {
		t.Fatalf("script or style content survived:\n%s", rendered)
	}
	if !strings.Contains(rendered, "First paragraph.") || !strings.Contains(rendered, "Second paragraph.") {
		t.Fatalf("paragraph text was lost:\n%s", rendered)
	}
	if !strings.Contains(rendered, "First paragraph.\n\nSecond paragraph.") {
		t.Fatalf("paragraphs were not separated by a blank line:\n%s", rendered)
	}
}

func TestHTMLDocumentKeepsInlineSpacingAndLists(t *testing.T) {
	var rendered string

	rendered = renderHTML(t, `<body><p>a <strong>useful</strong> page</p><ul><li>one</li><li>two</li></ul></body>`)

	if !strings.Contains(rendered, "a useful page") {
		t.Fatalf("inline markup glued words together:\n%s", rendered)
	}
	if !strings.Contains(rendered, "- one") || !strings.Contains(rendered, "- two") {
		t.Fatalf("list items lost their markers:\n%s", rendered)
	}
}

func TestFetchRenderPassesStructuredBodiesThrough(t *testing.T) {
	var body string

	var err error

	body, err = fetchRender("application/json", []byte(`{"ok":true}`), false)
	if err != nil || body != `{"ok":true}` {
		t.Fatalf("json render = %q, %v", body, err)
	}

	body, err = fetchRender("text/plain", []byte("plain text"), false)
	if err != nil || body != "plain text" {
		t.Fatalf("text render = %q, %v", body, err)
	}

	body, err = fetchRender("text/html", []byte("<p>hi</p>"), true)
	if err != nil || body != "<p>hi</p>" {
		t.Fatalf("raw render = %q, %v", body, err)
	}

	body, err = fetchRender("image/png", []byte{1, 2, 3, 4}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "binary content omitted") || strings.Contains(body, "\x01") {
		t.Fatalf("binary render = %q", body)
	}
}

func TestFetchFormatHeaderAndTruncation(t *testing.T) {
	var output string

	output = fetchFormat(404, "https://example.com/missing", "text/html; charset=utf-8", "not here", 100)
	if !strings.HasPrefix(output, "url: https://example.com/missing\nstatus: 404\ncontent-type: text/html; charset=utf-8\n\n") {
		t.Fatalf("header block = %q", output)
	}
	if strings.Contains(output, "[truncated]") {
		t.Fatalf("a short body was truncated: %q", output)
	}

	output = fetchFormat(200, "https://example.com/", "text/plain", "abcdefghij", 4)
	if !strings.HasSuffix(output, "abcd\n[truncated]") {
		t.Fatalf("truncation = %q", output)
	}
}

func TestFetchFormatTruncatesOnRunes(t *testing.T) {
	var body string
	var output string

	body = strings.Repeat("한", 10)

	output = fetchFormat(200, "https://example.com/", "text/plain", body, 4)
	if !strings.HasSuffix(output, "한한한한\n[truncated]") {
		t.Fatalf("multibyte truncation = %q", output)
	}
	if !strings.Contains(output, "한한한한") || strings.Contains(output, "�") {
		t.Fatalf("truncation split a multibyte character: %q", output)
	}
}
