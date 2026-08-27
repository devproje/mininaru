// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"strings"

	"golang.org/x/net/html"
)

var htmlSkippedTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"svg": true, "head": true, "iframe": true, "object": true,
}

var htmlBlockTags = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "hr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"section": true, "article": true, "header": true, "footer": true, "nav": true,
	"blockquote": true, "pre": true, "table": true, "ul": true, "ol": true,
	"dl": true, "dt": true, "dd": true, "form": true,
}

func htmlRender(node *html.Node, body *strings.Builder) {
	var text string
	var block bool
	var child *html.Node

	if node.Type == html.ElementNode && htmlSkippedTags[node.Data] {
		return
	}

	if node.Type == html.TextNode {
		text = strings.Join(strings.Fields(node.Data), " ")
		if text == "" {
			return
		}

		if strings.HasPrefix(node.Data, " ") || strings.HasPrefix(node.Data, "\n") || strings.HasPrefix(node.Data, "\t") {
			body.WriteString(" ")
		}

		body.WriteString(text)

		if strings.HasSuffix(node.Data, " ") || strings.HasSuffix(node.Data, "\n") || strings.HasSuffix(node.Data, "\t") {
			body.WriteString(" ")
		}

		return
	}

	block = node.Type == html.ElementNode && htmlBlockTags[node.Data]
	if block {
		body.WriteString("\n")
	}
	if node.Type == html.ElementNode && node.Data == "li" {
		body.WriteString("- ")
	}

	for child = node.FirstChild; child != nil; child = child.NextSibling {
		htmlRender(child, body)
	}

	if block {
		body.WriteString("\n")
	}
}

func htmlDocument(node *html.Node) string {
	var body strings.Builder
	var lines []string
	var line string
	var blanks int
	var cleaned []string

	htmlRender(node, &body)

	lines = strings.Split(body.String(), "\n")

	for _, line = range lines {
		line = strings.TrimRight(strings.Join(strings.Fields(line), " "), " ")
		if line == "" {
			blanks++
			if blanks > 1 {
				continue
			}

			cleaned = append(cleaned, line)
			continue
		}

		blanks = 0
		cleaned = append(cleaned, line)
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
