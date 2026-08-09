// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

const defaultFetchChars = 24576
const maxFetchChars = 262144
const maxFetchResponseBytes = 2097152

func fetchTextual(mediaType string) bool {
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	if strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml") {
		return true
	}

	switch mediaType {
	case "application/json", "application/xml", "application/javascript", "application/x-ndjson":
		return true
	}

	return false
}

func fetchMarkup(mediaType string) bool {
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

func fetchRender(mediaType string, body []byte, raw bool) (string, error) {
	var document *html.Node

	var err error

	if fetchMarkup(mediaType) && !raw {
		document, err = html.Parse(strings.NewReader(string(body)))
		if err != nil {
			return "", err
		}

		return htmlDocument(document), nil
	}

	if raw || fetchTextual(mediaType) {
		return string(body), nil
	}

	return fmt.Sprintf("[binary content omitted: %s, %d bytes]", mediaType, len(body)), nil
}

func fetchFormat(status int, finalURL, contentType, body string, maxChars int) string {
	var builder strings.Builder
	var runes []rune

	builder.WriteString("url: " + finalURL + "\n")
	builder.WriteString(fmt.Sprintf("status: %d\n", status))
	if contentType != "" {
		builder.WriteString("content-type: " + contentType + "\n")
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return strings.TrimRight(builder.String(), "\n")
	}

	builder.WriteString("\n")

	runes = []rune(body)
	if len(runes) <= maxChars {
		builder.WriteString(body)

		return builder.String()
	}

	builder.WriteString(string(runes[:maxChars]))
	builder.WriteString("\n[truncated]")

	return builder.String()
}

func WebFetch() Def {
	return Def{
		Name:        "web_fetch",
		Description: "Fetch a public http or https url and return its text. Private and local addresses are refused.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":       map[string]any{"type": "string", "description": "Absolute http or https url to fetch."},
				"raw":       map[string]any{"type": "boolean", "description": "Return the body unprocessed instead of extracting text from html."},
				"max_chars": map[string]any{"type": "integer", "minimum": 1, "maximum": maxFetchChars},
			},
			"required":             []string{"url"},
			"additionalProperties": false,
		},
		Permission: PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				URL      string `json:"url"`
				Raw      bool   `json:"raw"`
				MaxChars int    `json:"max_chars"`
			}
			var request *http.Request
			var response *http.Response
			var buf []byte
			var contentType string
			var mediaType string
			var body string

			var err error

			err = ctx.Err()
			if err != nil {
				return "", err
			}

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			payload.URL = strings.TrimSpace(payload.URL)
			if payload.URL == "" {
				return "", fmt.Errorf("url is required")
			}
			if payload.MaxChars <= 0 {
				payload.MaxChars = defaultFetchChars
			}
			if payload.MaxChars > maxFetchChars {
				return "", fmt.Errorf("max_chars cannot exceed %d", maxFetchChars)
			}

			_, err = fetchTarget(payload.URL)
			if err != nil {
				return "", fmt.Errorf("web_fetch refused %s: %w", payload.URL, err)
			}

			request, err = http.NewRequestWithContext(ctx, http.MethodGet, payload.URL, nil)
			if err != nil {
				return "", err
			}
			request.Header.Set("User-Agent", "mininaru web_fetch")
			request.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,application/json;q=0.9,*/*;q=0.8")

			response, err = webFetchClient.Do(request)
			if err != nil {
				if errors.Is(err, errBlockedAddress) {
					return "", fmt.Errorf("web_fetch refused %s: %w", payload.URL, errBlockedAddress)
				}

				return "", err
			}
			defer response.Body.Close()

			buf, err = io.ReadAll(io.LimitReader(response.Body, maxFetchResponseBytes))
			if err != nil {
				return "", err
			}

			contentType = response.Header.Get("Content-Type")

			mediaType, _, err = mime.ParseMediaType(contentType)
			if err != nil {
				mediaType = "application/octet-stream"
			}

			body, err = fetchRender(mediaType, buf, payload.Raw)
			if err != nil {
				return "", err
			}

			return fetchFormat(response.StatusCode, response.Request.URL.String(), contentType, body, payload.MaxChars), nil
		},
	}
}
