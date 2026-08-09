// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var htmlSkippedTags map[string]bool = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"svg": true, "head": true, "iframe": true, "object": true,
}

var htmlBlockTags map[string]bool = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "hr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"section": true, "article": true, "header": true, "footer": true, "nav": true,
	"blockquote": true, "pre": true, "table": true, "ul": true, "ol": true,
	"dl": true, "dt": true, "dd": true, "form": true,
}

func htmlAttribute(node *html.Node, name string) string {
	var attribute html.Attribute

	for _, attribute = range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}

	return ""
}

func htmlClass(node *html.Node, name string) bool {
	var classes []string
	var class string

	classes = strings.Fields(htmlAttribute(node, "class"))
	for _, class = range classes {
		if class == name {
			return true
		}
	}

	return false
}

func htmlTextInto(node *html.Node, body *strings.Builder) {
	var child *html.Node

	if node.Type == html.TextNode {
		body.WriteString(node.Data)
	}
	for child = node.FirstChild; child != nil; child = child.NextSibling {
		htmlTextInto(child, body)
	}
}

func htmlText(node *html.Node) string {
	var body strings.Builder

	htmlTextInto(node, &body)

	return strings.Join(strings.Fields(body.String()), " ")
}

func findDescendant(node *html.Node, tag, class string) *html.Node {
	var child *html.Node
	var found *html.Node

	if node.Type == html.ElementNode && node.Data == tag && htmlClass(node, class) {
		return node
	}
	for child = node.FirstChild; child != nil; child = child.NextSibling {
		found = findDescendant(child, tag, class)
		if found != nil {
			return found
		}
	}

	return nil
}

func htmlInline(raw string) string {
	var document *html.Node

	var err error

	if !strings.Contains(raw, "<") {
		return strings.Join(strings.Fields(raw), " ")
	}

	document, err = html.Parse(strings.NewReader(raw))
	if err != nil {
		return strings.Join(strings.Fields(raw), " ")
	}

	return htmlText(document)
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

func httpURL(raw string) string {
	var parsed *url.URL

	var err error

	parsed, err = url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}

	return raw
}

func duckURL(raw string) string {
	var parsed *url.URL
	var target string

	var err error

	parsed, err = url.Parse(raw)
	if err != nil {
		return ""
	}

	target = parsed.Query().Get("uddg")
	if target != "" {
		return httpURL(target)
	}

	return httpURL(raw)
}
