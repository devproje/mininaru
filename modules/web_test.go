package modules

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebSearchParsesResults(t *testing.T) {
	var server *httptest.Server
	var previousEndpoint string
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

	previousEndpoint = webSearchEndpoint
	webSearchEndpoint = server.URL
	defer func() { webSearchEndpoint = previousEndpoint }()

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
