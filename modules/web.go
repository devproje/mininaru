// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type searchProvider struct {
	Name    string
	Request func(ctx context.Context, cfg *SearchConfig, query string, limit int) (*http.Request, error)
	Parse   func(body io.Reader, limit int) ([]SearchResult, error)
}

const defaultSearchLimit = 5
const maxSearchLimit = 10
const maxSearchResponseBytes = 2097152

var searchEndpoints map[string]string = map[string]string{
	ProviderDuckDuckGo: "https://html.duckduckgo.com/html/",
	ProviderBrave:      "https://api.search.brave.com/res/v1/web/search",
	ProviderTavily:     "https://api.tavily.com/search",
}

var webSearchClient *http.Client = &http.Client{Timeout: 20 * time.Second}

func searchEndpoint(cfg *SearchConfig) string {
	if cfg.Endpoint != "" {
		return cfg.Endpoint
	}

	return searchEndpoints[cfg.Provider]
}

func collectSearchResults(node *html.Node, limit int, results *[]SearchResult) {
	var anchor *html.Node
	var snippet *html.Node
	var item SearchResult
	var child *html.Node

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
			item = SearchResult{Title: htmlText(anchor), URL: duckURL(htmlAttribute(anchor, "href"))}
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

func duckRequest(ctx context.Context, cfg *SearchConfig, query string, limit int) (*http.Request, error) {
	var form url.Values
	var request *http.Request

	var err error

	form = url.Values{"q": []string{query}}

	request, err = http.NewRequestWithContext(ctx, http.MethodPost, searchEndpoint(cfg), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "mininaru web_search")

	return request, nil
}

func duckParse(body io.Reader, limit int) ([]SearchResult, error) {
	var document *html.Node
	var results []SearchResult

	var err error

	document, err = html.Parse(body)
	if err != nil {
		return nil, err
	}

	collectSearchResults(document, limit, &results)

	return results, nil
}

func searxRequest(ctx context.Context, cfg *SearchConfig, query string, limit int) (*http.Request, error) {
	var endpoint string
	var target string
	var request *http.Request

	var err error

	endpoint = strings.TrimSuffix(searchEndpoint(cfg), "/")
	if !strings.HasSuffix(endpoint, "/search") {
		endpoint = endpoint + "/search"
	}

	target = endpoint + "?" + url.Values{
		"q": []string{query}, "format": []string{"json"}, "safesearch": []string{"1"},
	}.Encode()

	request, err = http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "mininaru web_search")

	return request, nil
}

func searxParse(body io.Reader, limit int) ([]SearchResult, error) {
	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	var index int
	var target string
	var results []SearchResult

	var err error

	err = json.NewDecoder(body).Decode(&payload)
	if err != nil {
		return nil, err
	}

	for index = range payload.Results {
		target = httpURL(payload.Results[index].URL)
		if target == "" || payload.Results[index].Title == "" {
			continue
		}

		results = append(results, SearchResult{
			Title: payload.Results[index].Title, URL: target, Snippet: payload.Results[index].Content,
		})
	}

	return results, nil
}

func braveRequest(ctx context.Context, cfg *SearchConfig, query string, limit int) (*http.Request, error) {
	var target string
	var request *http.Request

	var err error

	target = searchEndpoint(cfg) + "?" + url.Values{
		"q": []string{query}, "count": []string{strconv.Itoa(limit)},
	}.Encode()

	request, err = http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Subscription-Token", cfg.APIKey)

	return request, nil
}

func braveParse(body io.Reader, limit int) ([]SearchResult, error) {
	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	var index int
	var target string
	var results []SearchResult

	var err error

	err = json.NewDecoder(body).Decode(&payload)
	if err != nil {
		return nil, err
	}

	for index = range payload.Web.Results {
		target = httpURL(payload.Web.Results[index].URL)
		if target == "" || payload.Web.Results[index].Title == "" {
			continue
		}

		results = append(results, SearchResult{
			Title:   htmlInline(payload.Web.Results[index].Title),
			URL:     target,
			Snippet: htmlInline(payload.Web.Results[index].Description),
		})
	}

	return results, nil
}

func tavilyRequest(ctx context.Context, cfg *SearchConfig, query string, limit int) (*http.Request, error) {
	var payload map[string]any
	var buf []byte
	var request *http.Request

	var err error

	payload = map[string]any{"query": query, "max_results": limit, "search_depth": "basic"}

	buf, err = json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodPost, searchEndpoint(cfg), strings.NewReader(string(buf)))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	return request, nil
}

func tavilyParse(body io.Reader, limit int) ([]SearchResult, error) {
	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	var index int
	var target string
	var results []SearchResult

	var err error

	err = json.NewDecoder(body).Decode(&payload)
	if err != nil {
		return nil, err
	}

	for index = range payload.Results {
		target = httpURL(payload.Results[index].URL)
		if target == "" || payload.Results[index].Title == "" {
			continue
		}

		results = append(results, SearchResult{
			Title: payload.Results[index].Title, URL: target, Snippet: payload.Results[index].Content,
		})
	}

	return results, nil
}

func searchProviders() []searchProvider {
	return []searchProvider{
		{Name: ProviderDuckDuckGo, Request: duckRequest, Parse: duckParse},
		{Name: ProviderSearXNG, Request: searxRequest, Parse: searxParse},
		{Name: ProviderBrave, Request: braveRequest, Parse: braveParse},
		{Name: ProviderTavily, Request: tavilyRequest, Parse: tavilyParse},
	}
}

func searchFind(name string) *searchProvider {
	var providers []searchProvider
	var index int

	providers = searchProviders()

	for index = range providers {
		if providers[index].Name != name {
			continue
		}

		return &providers[index]
	}

	return nil
}

func searchRun(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	var cfg SearchConfig
	var provider *searchProvider
	var request *http.Request
	var response *http.Response
	var results []SearchResult

	var err error

	cfg = WebSearchConfig()

	provider = searchFind(cfg.Provider)
	if provider == nil {
		return nil, fmt.Errorf("unknown search provider %q", cfg.Provider)
	}

	request, err = provider.Request(ctx, &cfg, query, limit)
	if err != nil {
		return nil, err
	}

	response, err = webSearchClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s search returned HTTP %d", cfg.Provider, response.StatusCode)
	}

	results, err = provider.Parse(io.LimitReader(response.Body, maxSearchResponseBytes), limit)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("search returned no results")
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
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

			results, err = searchRun(ctx, payload.Query, payload.Limit)
			if err != nil {
				return "", err
			}

			buf, err = json.Marshal(results)
			if err != nil {
				return "", err
			}

			return string(buf), nil
		},
	}
}
