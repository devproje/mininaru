// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func requireChrome(t *testing.T) {
	t.Helper()

	if !Available() {
		t.Skip("no Chrome/Chromium binary found, set MININARU_CHROME or install one to run this test")
	}
}

func testPage(t *testing.T) string {
	var server *httptest.Server

	t.Helper()

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><html><head><title>fixture</title></head>
<body>
<p id="greeting">hello there</p>
<input id="box" type="text">
</body></html>`))
	}))
	t.Cleanup(server.Close)

	return server.URL
}

func TestBrowserToolsFullRoundtrip(t *testing.T) {
	var sessionId string
	var url string
	var result string

	var err error

	requireChrome(t)

	sessionId = "test-session-1"
	url = testPage(t)
	t.Cleanup(func() { closeSession(sessionId) })

	result, err = navigate(sessionId).Execute(context.Background(), toArgs(t, map[string]string{"url": url}))
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if !strings.Contains(result, "fixture") {
		t.Fatalf("navigate result missing title: %q", result)
	}

	result, err = read(sessionId).Execute(context.Background(), toArgs(t, map[string]string{"selector": "#greeting"}))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(result, "hello there") {
		t.Fatalf("read result = %q, want it to contain the greeting text", result)
	}

	result, err = typeText(sessionId).Execute(context.Background(), toArgs(t, map[string]string{"selector": "#box", "text": "hi"}))
	if err != nil {
		t.Fatalf("type failed: %v", err)
	}

	result, err = click(sessionId).Execute(context.Background(), toArgs(t, map[string]string{"selector": "#greeting"}))
	if err != nil {
		t.Fatalf("click failed: %v", err)
	}

	result, err = screenshot(sessionId).Execute(context.Background(), "{}")
	if err != nil {
		t.Fatalf("screenshot failed: %v", err)
	}
	if !strings.HasPrefix(result, "data:image/png;base64,") {
		t.Fatalf("screenshot result missing the data URL prefix: %q", result[:min(40, len(result))])
	}

	result, err = closeTool(sessionId).Execute(context.Background(), "{}")
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func toArgs(t *testing.T, payload map[string]string) string {
	var buf []byte

	var err error

	t.Helper()

	buf, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	return string(buf)
}
