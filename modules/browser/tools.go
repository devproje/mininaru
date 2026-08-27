// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/devproje/mininaru/modules"
	"golang.org/x/net/html"
)

var callTimeout = 30 * time.Second

func navigate(sessionId string) modules.Tool {
	return modules.Tool{
		Name:        "browser_navigate",
		Description: "Navigate this session's browser tab to a URL. Creates the tab on first use.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string"},
			},
			"required":             []string{"url"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Url string `json:"url"`
			}
			var callCtx context.Context
			var cancel context.CancelFunc
			var location string
			var title string

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Url == "" {
				return "", fmt.Errorf("url is required")
			}

			callCtx, cancel, err = withCallTimeout(sessionId)
			if err != nil {
				return "", err
			}
			defer cancel()

			err = chromedp.Run(callCtx,
				chromedp.Navigate(payload.Url),
				chromedp.Location(&location),
				chromedp.Title(&title),
			)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("url: %s\ntitle: %s", location, title), nil
		},
	}
}

func click(sessionId string) modules.Tool {
	return modules.Tool{
		Name:        "browser_click",
		Description: "Click an element in this session's browser tab, identified by a CSS selector.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string"},
			},
			"required":             []string{"selector"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Selector string `json:"selector"`
			}
			var callCtx context.Context
			var cancel context.CancelFunc

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Selector == "" {
				return "", fmt.Errorf("selector is required")
			}

			callCtx, cancel, err = withCallTimeout(sessionId)
			if err != nil {
				return "", err
			}
			defer cancel()

			err = chromedp.Run(callCtx, chromedp.Click(payload.Selector, chromedp.ByQuery))
			if err != nil {
				return "", err
			}

			return "clicked " + payload.Selector, nil
		},
	}
}

func typeText(sessionId string) modules.Tool {
	return modules.Tool{
		Name:        "browser_type",
		Description: "Focus an element by CSS selector and type text into it, in this session's browser tab.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string"},
				"text":     map[string]any{"type": "string"},
			},
			"required":             []string{"selector", "text"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Selector string `json:"selector"`
				Text     string `json:"text"`
			}
			var callCtx context.Context
			var cancel context.CancelFunc

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Selector == "" {
				return "", fmt.Errorf("selector is required")
			}

			callCtx, cancel, err = withCallTimeout(sessionId)
			if err != nil {
				return "", err
			}
			defer cancel()

			err = chromedp.Run(callCtx,
				chromedp.Focus(payload.Selector, chromedp.ByQuery),
				chromedp.SendKeys(payload.Selector, payload.Text, chromedp.ByQuery),
			)
			if err != nil {
				return "", err
			}

			return "typed into " + payload.Selector, nil
		},
	}
}

func read(sessionId string) modules.Tool {
	return modules.Tool{
		Name:        "browser_read",
		Description: "Read the visible text of the current page (or one element by CSS selector) in this session's browser tab.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string"},
			},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Selector string `json:"selector"`
			}
			var callCtx context.Context
			var cancel context.CancelFunc
			var selector string
			var outer string
			var document *html.Node

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			selector = payload.Selector
			if selector == "" {
				selector = "html"
			}

			callCtx, cancel, err = withCallTimeout(sessionId)
			if err != nil {
				return "", err
			}
			defer cancel()

			err = chromedp.Run(callCtx, chromedp.OuterHTML(selector, &outer, chromedp.ByQuery))
			if err != nil {
				return "", err
			}

			document, err = html.Parse(strings.NewReader(outer))
			if err != nil {
				return "", err
			}

			return htmlDocument(document), nil
		},
	}
}

func screenshot(sessionId string) modules.Tool {
	return modules.Tool{
		Name:        "browser_screenshot",
		Description: "Capture a screenshot of the current page in this session's browser tab.",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var callCtx context.Context
			var cancel context.CancelFunc
			var buf []byte

			var err error

			callCtx, cancel, err = withCallTimeout(sessionId)
			if err != nil {
				return "", err
			}
			defer cancel()

			err = chromedp.Run(callCtx, chromedp.CaptureScreenshot(&buf))
			if err != nil {
				return "", err
			}

			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf), nil
		},
	}
}

func closeTool(sessionId string) modules.Tool {
	return modules.Tool{
		Name:        "browser_close",
		Description: "Close this session's browser tab.",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			closeSession(sessionId)

			return "browser closed", nil
		},
	}
}

func Tools(sessionId string) []modules.Tool {
	return []modules.Tool{
		navigate(sessionId),
		click(sessionId),
		typeText(sessionId),
		read(sessionId),
		screenshot(sessionId),
		closeTool(sessionId),
	}
}
