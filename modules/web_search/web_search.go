// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package web_search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devproje/mininaru/modules"
)

type braveResponse struct {
	Web struct {
		Results []braveHit `json:"results"`
	} `json:"web"`
}

type braveHit struct {
	Title       string `json:"title"`
	Url         string `json:"url"`
	Description string `json:"description"`
}

type tavilyResponse struct {
	Results []tavilyHit `json:"results"`
}

type tavilyHit struct {
	Title   string `json:"title"`
	Url     string `json:"url"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Results []ollamaHit `json:"results"`
}

type ollamaHit struct {
	Title   string `json:"title"`
	Url     string `json:"url"`
	Content string `json:"content"`
}

type searchResult struct {
	Title   string
	Url     string
	Snippet string
}

const defaultMaxResults = 5
const maxAllowedResults = 20
const requestTimeout = 15 * time.Second
const maxResponseBytes = 2 << 20

const braveDefaultUrl = "https://api.search.brave.com/res/v1/web/search"
const tavilyDefaultUrl = "https://api.tavily.com/search"
const ollamaDefaultUrl = "https://ollama.com/api/web_search"

func formatResults(results []searchResult) string {
	var body strings.Builder
	var index int
	var result searchResult

	if len(results) == 0 {
		return "no results"
	}

	for index, result = range results {
		body.WriteString(fmt.Sprintf("%d. %s\n   %s\n", index+1, result.Title, result.Url))
		if result.Snippet != "" {
			body.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
		}
	}

	return strings.TrimRight(body.String(), "\n")
}

func braveSearch(ctx context.Context, backend *modules.WebBackend, query string, maxResults int) ([]searchResult, error) {
	var endpoint string
	var request *http.Request
	var client http.Client
	var response *http.Response
	var body []byte
	var payload braveResponse
	var results []searchResult
	var hit braveHit

	var err error

	endpoint = backend.BaseUrl
	if endpoint == "" {
		endpoint = braveDefaultUrl
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?q=%s&count=%d", endpoint, url.QueryEscape(query), maxResults), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Subscription-Token", backend.ApiKey)

	client = http.Client{Timeout: requestTimeout}

	response, err = client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave search failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	err = json.Unmarshal(body, &payload)
	if err != nil {
		return nil, err
	}

	for _, hit = range payload.Web.Results {
		results = append(results, searchResult{Title: hit.Title, Url: hit.Url, Snippet: hit.Description})
	}

	return results, nil
}

func tavilySearch(ctx context.Context, backend *modules.WebBackend, query string, maxResults int) ([]searchResult, error) {
	var endpoint string
	var requestBody []byte
	var request *http.Request
	var client http.Client
	var response *http.Response
	var body []byte
	var payload tavilyResponse
	var results []searchResult
	var hit tavilyHit

	var err error

	endpoint = backend.BaseUrl
	if endpoint == "" {
		endpoint = tavilyDefaultUrl
	}

	requestBody, err = json.Marshal(map[string]any{
		"api_key":     backend.ApiKey,
		"query":       query,
		"max_results": maxResults,
	})
	if err != nil {
		return nil, err
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	client = http.Client{Timeout: requestTimeout}

	response, err = client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily search failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	err = json.Unmarshal(body, &payload)
	if err != nil {
		return nil, err
	}

	for _, hit = range payload.Results {
		results = append(results, searchResult{Title: hit.Title, Url: hit.Url, Snippet: hit.Content})
	}

	return results, nil
}

func ollamaSearch(ctx context.Context, backend *modules.WebBackend, query string) ([]searchResult, error) {
	var endpoint string
	var requestBody []byte
	var request *http.Request
	var client http.Client
	var response *http.Response
	var body []byte
	var payload ollamaResponse
	var results []searchResult
	var hit ollamaHit

	var err error

	endpoint = backend.BaseUrl
	if endpoint == "" {
		endpoint = ollamaDefaultUrl
	}

	requestBody, err = json.Marshal(map[string]any{"query": query})
	if err != nil {
		return nil, err
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+backend.ApiKey)

	client = http.Client{Timeout: requestTimeout}

	response, err = client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama search failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	err = json.Unmarshal(body, &payload)
	if err != nil {
		return nil, err
	}

	for _, hit = range payload.Results {
		results = append(results, searchResult{Title: hit.Title, Url: hit.Url, Snippet: hit.Content})
	}

	return results, nil
}

func Search(lookup modules.WebBackendLookup) modules.Tool {
	return modules.Tool{
		Name:        "web_search",
		Description: "Search the web using the configured provider (Brave, Tavily, or Ollama Search) and return matching results.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":       map[string]any{"type": "string"},
				"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": maxAllowedResults},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Query      string `json:"query"`
				MaxResults int    `json:"max_results"`
			}
			var backend *modules.WebBackend
			var results []searchResult

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Query == "" {
				return "", fmt.Errorf("query is required")
			}
			if payload.MaxResults <= 0 {
				payload.MaxResults = defaultMaxResults
			}
			if payload.MaxResults > maxAllowedResults {
				payload.MaxResults = maxAllowedResults
			}

			backend, err = lookup()
			if err != nil {
				return "", err
			}

			switch backend.Kind {
			case modules.WebBackendBrave:
				results, err = braveSearch(ctx, backend, payload.Query, payload.MaxResults)
			case modules.WebBackendTavily:
				results, err = tavilySearch(ctx, backend, payload.Query, payload.MaxResults)
			case modules.WebBackendOllama:
				results, err = ollamaSearch(ctx, backend, payload.Query)
			default:
				err = fmt.Errorf("unsupported web search backend %q", backend.Kind)
			}
			if err != nil {
				return "", err
			}

			if len(results) > payload.MaxResults {
				results = results[:payload.MaxResults]
			}

			return formatResults(results), nil
		},
	}
}

func Tools(lookup modules.WebBackendLookup) []modules.Tool {
	return []modules.Tool{Search(lookup)}
}
