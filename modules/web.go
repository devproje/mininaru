package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

const defaultSearchLimit = 5
const maxSearchLimit = 10
const maxSearchResponseBytes = 2097152

var webSearchEndpoint string = "https://html.duckduckgo.com/html/"
var webSearchClient *http.Client = &http.Client{Timeout: 20 * time.Second}

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

func htmlText(node *html.Node) string {
	var body strings.Builder
	var child *html.Node

	if node.Type == html.TextNode {
		body.WriteString(node.Data)
	}
	for child = node.FirstChild; child != nil; child = child.NextSibling {
		body.WriteString(htmlText(child))
	}

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

func resultURL(raw string) string {
	var parsed *url.URL
	var target string
	var targetURL *url.URL
	var err error

	parsed, err = url.Parse(raw)
	if err != nil {
		return raw
	}
	target = parsed.Query().Get("uddg")
	if target != "" {
		targetURL, err = url.Parse(target)
		if err == nil && (targetURL.Scheme == "http" || targetURL.Scheme == "https") {
			return target
		}
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}

	return raw
}

func collectSearchResults(node *html.Node, limit int, results *[]SearchResult) {
	var child *html.Node
	var anchor *html.Node
	var snippet *html.Node
	var item SearchResult

	if len(*results) >= limit {
		return
	}
	if node.Type == html.ElementNode && htmlClass(node, "result") {
		anchor = findDescendant(node, "a", "result__a")
		if anchor != nil {
			snippet = findDescendant(node, "a", "result__snippet")
			if snippet == nil {
				snippet = findDescendant(node, "div", "result__snippet")
			}
			item = SearchResult{Title: htmlText(anchor), URL: resultURL(htmlAttribute(anchor, "href"))}
			if snippet != nil {
				item.Snippet = htmlText(snippet)
			}
			if item.Title != "" && item.URL != "" {
				*results = append(*results, item)
			}
		}
	}
	for child = node.FirstChild; child != nil && len(*results) < limit; child = child.NextSibling {
		collectSearchResults(child, limit, results)
	}
}

func WebSearch() Def {
	return Def{
		Name:        "web_search",
		Description: "Search the public web and return result titles, URLs, and snippets.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": maxSearchLimit},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		},
		Permission: PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			var form url.Values
			var request *http.Request
			var response *http.Response
			var document *html.Node
			var results []SearchResult
			var buf []byte

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			payload.Query = strings.TrimSpace(payload.Query)
			if payload.Query == "" {
				return "", fmt.Errorf("query is required")
			}
			if payload.Limit <= 0 {
				payload.Limit = defaultSearchLimit
			}
			if payload.Limit > maxSearchLimit {
				return "", fmt.Errorf("limit cannot exceed %d", maxSearchLimit)
			}

			form = url.Values{"q": []string{payload.Query}}
			request, err = http.NewRequestWithContext(ctx, http.MethodPost, webSearchEndpoint, strings.NewReader(form.Encode()))
			if err != nil {
				return "", err
			}
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("User-Agent", "mininaru web_search")

			response, err = webSearchClient.Do(request)
			if err != nil {
				return "", err
			}
			defer response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				return "", fmt.Errorf("search returned HTTP %d", response.StatusCode)
			}

			document, err = html.Parse(io.LimitReader(response.Body, maxSearchResponseBytes))
			if err != nil {
				return "", err
			}
			collectSearchResults(document, payload.Limit, &results)
			if len(results) == 0 {
				return "", fmt.Errorf("search returned no results")
			}

			buf, err = json.Marshal(results)
			if err != nil {
				return "", err
			}

			return string(buf), nil
		},
	}
}
