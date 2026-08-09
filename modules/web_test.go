// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func webTestConfig(t *testing.T, provider, endpoint, key string) {
	var previous SearchConfig

	t.Helper()

	previous = WebSearchConfig()
	t.Cleanup(func() { WebSetSearch(previous) })

	WebSetSearch(SearchConfig{Provider: provider, Endpoint: endpoint, APIKey: key})
}

func TestWebSearchParsesResults(t *testing.T) {
	var server *httptest.Server
	var requestBody string
	var result string

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requestBody = string(body)
		io.WriteString(w, `<html><body><div class="result"><a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fdoc">Example</a><a class="result__snippet">Useful result</a></div></body></html>`)
	}))
	defer server.Close()

	webTestConfig(t, ProviderDuckDuckGo, server.URL, "")

	result, err = WebSearch().Execute(context.Background(), `{"query":"mininaru","limit":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestBody, "q=mininaru") {
		t.Fatalf("search form = %q", requestBody)
	}
	if !strings.Contains(result, `"title":"Example"`) || !strings.Contains(result, `"url":"https://example.com/doc"`) {
		t.Fatalf("search result = %q", result)
	}
}

func TestWebSearchSearXNG(t *testing.T) {
	var server *httptest.Server
	var query string
	var format string
	var result string

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("q")
		format = r.URL.Query().Get("format")

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"results":[{"title":"Example","url":"https://example.com/doc","content":"Useful result"},
			{"title":"Bad","url":"file:///etc/passwd","content":"nope"}]}`)
	}))
	defer server.Close()

	webTestConfig(t, ProviderSearXNG, server.URL, "")

	result, err = WebSearch().Execute(context.Background(), `{"query":"mininaru","limit":5}`)
	if err != nil {
		t.Fatal(err)
	}
	if query != "mininaru" || format != "json" {
		t.Fatalf("query = %q, format = %q", query, format)
	}
	if !strings.Contains(result, `"snippet":"Useful result"`) {
		t.Fatalf("result = %q", result)
	}
	if strings.Contains(result, "passwd") {
		t.Fatalf("a non-http result url survived: %q", result)
	}
}

func TestWebSearchBrave(t *testing.T) {
	var server *httptest.Server
	var token string
	var result string

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token = r.Header.Get("X-Subscription-Token")

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"web":{"results":[{"title":"Example","url":"https://example.com/doc","description":"a <strong>useful</strong> page"}]}}`)
	}))
	defer server.Close()

	webTestConfig(t, ProviderBrave, server.URL, "secret-key")

	result, err = WebSearch().Execute(context.Background(), `{"query":"mininaru"}`)
	if err != nil {
		t.Fatal(err)
	}
	if token != "secret-key" {
		t.Fatalf("subscription token = %q", token)
	}
	if !strings.Contains(result, `"snippet":"a useful page"`) {
		t.Fatalf("highlight markup survived: %q", result)
	}
}

func TestWebSearchTavily(t *testing.T) {
	var server *httptest.Server
	var requestBody string
	var authorization string
	var result string

	var err error

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, _ = io.ReadAll(r.Body)
		requestBody = string(body)
		authorization = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"results":[{"title":"Example","url":"https://example.com/doc","content":"Useful result"}]}`)
	}))
	defer server.Close()

	webTestConfig(t, ProviderTavily, server.URL, "tvly-key")

	result, err = WebSearch().Execute(context.Background(), `{"query":"mininaru","limit":3}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestBody, `"query":"mininaru"`) || !strings.Contains(requestBody, `"max_results":3`) {
		t.Fatalf("request body = %q", requestBody)
	}
	if authorization != "Bearer tvly-key" {
		t.Fatalf("authorization = %q", authorization)
	}
	if !strings.Contains(result, `"url":"https://example.com/doc"`) {
		t.Fatalf("result = %q", result)
	}
}

func TestWebConfigFallsBackOnBadProvider(t *testing.T) {
	var accepted WebConfig

	accepted = webAccept(WebConfig{Search: SearchConfig{Provider: ProviderBrave}})
	if accepted.Search.Provider != ProviderDuckDuckGo {
		t.Fatalf("a keyless brave config was accepted: %#v", accepted)
	}

	accepted = webAccept(WebConfig{Search: SearchConfig{Provider: "altavista"}})
	if accepted.Search.Provider != ProviderDuckDuckGo {
		t.Fatalf("an unknown provider was accepted: %#v", accepted)
	}

	accepted = webAccept(WebConfig{Search: SearchConfig{Provider: ProviderSearXNG, Endpoint: "https://searx.example.com"}})
	if accepted.Search.Provider != ProviderSearXNG {
		t.Fatalf("a valid searxng config was rejected: %#v", accepted)
	}
}
