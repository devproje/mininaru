// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const testAPIKey = "test-key"

func setupTestDB(t *testing.T) {
	var err error

	t.Helper()

	gin.SetMode(gin.TestMode)

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.NewDatabase(util.Path("data.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		util.DB.Close()
	})
}

func TestNewAppServerWiresAPIRoutes(t *testing.T) {
	var app *AppServer
	var srv *httptest.Server
	var paths []string
	var path string
	var req *http.Request
	var resp *http.Response

	var err error

	setupTestDB(t)
	app = NewAppServer(Options{Host: "127.0.0.1", ApiKey: testAPIKey})
	srv = httptest.NewServer(app.WebServer.Handler)
	t.Cleanup(srv.Close)

	paths = []string{"/api/agents", "/api/providers", "/api/v1/models"}

	for _, path = range paths {
		req, err = http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		req.Header.Set("Authorization", "Bearer "+testAPIKey)

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestNewAppServerRejectsMissingOrWrongKey(t *testing.T) {
	var app *AppServer
	var srv *httptest.Server
	var req *http.Request
	var resp *http.Response

	var err error

	setupTestDB(t)
	app = NewAppServer(Options{Host: "127.0.0.1", ApiKey: testAPIKey})
	srv = httptest.NewServer(app.WebServer.Handler)
	t.Cleanup(srv.Close)

	resp, err = http.Get(srv.URL + "/api/agents")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/api/agents", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer wrong-key")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong key: status = %d, want 401", resp.StatusCode)
	}
}

func TestNewAppServerWiresWebSocket(t *testing.T) {
	var app *AppServer
	var srv *httptest.Server
	var wsURL string
	var header http.Header
	var conn *websocket.Conn
	var frame struct {
		Type string `json:"type"`
	}

	var err error

	setupTestDB(t)
	app = NewAppServer(Options{Host: "127.0.0.1", ApiKey: testAPIKey})
	srv = httptest.NewServer(app.WebServer.Handler)
	t.Cleanup(srv.Close)

	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	header = http.Header{"Authorization": []string{"Bearer " + testAPIKey}}

	conn, _, err = websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	err = conn.WriteMessage(websocket.TextMessage, []byte("not json"))
	if err != nil {
		t.Fatal(err)
	}

	err = conn.ReadJSON(&frame)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Type != "error" {
		t.Fatalf("type = %q, want error", frame.Type)
	}
}

func newTestServer(t *testing.T, options Options) *httptest.Server {
	var srv *httptest.Server

	t.Helper()

	setupTestDB(t)

	options.Host = "127.0.0.1"
	options.ApiKey = testAPIKey

	srv = httptest.NewServer(NewAppServer(options).WebServer.Handler)
	t.Cleanup(srv.Close)

	return srv
}

func doRequest(t *testing.T, method string, url string, header http.Header) (*http.Response, string) {
	var req *http.Request
	var resp *http.Response
	var body []byte

	var err error

	t.Helper()

	req, err = http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = header

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	return resp, string(body)
}

func TestCORSPreflightAllowedOriginSkipsAuth(t *testing.T) {
	var srv *httptest.Server
	var resp *http.Response

	srv = newTestServer(t, Options{CorsOrigins: []string{"tauri://localhost"}})

	resp, _ = doRequest(t, http.MethodOptions, srv.URL+"/api/agents", http.Header{
		"Origin":                        []string{"tauri://localhost"},
		"Access-Control-Request-Method": []string{"GET"},
	})

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "tauri://localhost" {
		t.Fatalf("allow-origin = %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("allow-headers = %q", resp.Header.Get("Access-Control-Allow-Headers"))
	}
}

func TestCORSDisallowedOriginGetsNoHeaders(t *testing.T) {
	var srv *httptest.Server
	var resp *http.Response

	srv = newTestServer(t, Options{CorsOrigins: []string{"tauri://localhost"}})

	resp, _ = doRequest(t, http.MethodOptions, srv.URL+"/api/agents", http.Header{
		"Origin": []string{"http://evil.example"},
	})

	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("allow-origin = %q, want empty", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if resp.StatusCode == http.StatusNoContent {
		t.Fatal("preflight from a disallowed origin must not succeed")
	}
}

func TestCORSNoOriginsConfiguredAddsNothing(t *testing.T) {
	var srv *httptest.Server
	var resp *http.Response

	srv = newTestServer(t, Options{})

	resp, _ = doRequest(t, http.MethodGet, srv.URL+"/api/agents", http.Header{
		"Authorization": []string{"Bearer " + testAPIKey},
		"Origin":        []string{"tauri://localhost"},
	})

	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("allow-origin = %q, want empty", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSActualRequestStillNeedsKey(t *testing.T) {
	var srv *httptest.Server
	var resp *http.Response

	srv = newTestServer(t, Options{CorsOrigins: []string{"tauri://localhost"}})

	resp, _ = doRequest(t, http.MethodGet, srv.URL+"/api/agents", http.Header{
		"Origin": []string{"tauri://localhost"},
	})

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "tauri://localhost" {
		t.Fatalf("a 401 must still carry allow-origin so the browser can read it, got %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestWebSocketAuthViaSubprotocol(t *testing.T) {
	var srv *httptest.Server
	var dialer websocket.Dialer
	var conn *websocket.Conn
	var resp *http.Response
	var wsURL string

	var err error

	srv = newTestServer(t, Options{})
	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	dialer = websocket.Dialer{Subprotocols: []string{"bearer." + testAPIKey}}
	conn, _, err = dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if conn.Subprotocol() != "bearer."+testAPIKey {
		t.Fatalf("subprotocol = %q", conn.Subprotocol())
	}

	dialer = websocket.Dialer{Subprotocols: []string{"bearer.wrong-key"}}
	conn, resp, err = dialer.Dial(wsURL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("wrong key must not upgrade")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong key response = %v", resp)
	}
}

func TestSubprotocolTokenIsIgnoredOutsideWebSocket(t *testing.T) {
	var srv *httptest.Server
	var resp *http.Response

	srv = newTestServer(t, Options{})

	resp, _ = doRequest(t, http.MethodGet, srv.URL+"/api/agents", http.Header{
		"Sec-WebSocket-Protocol": []string{"bearer." + testAPIKey},
	})

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestWebDirServesFilesAndFallsBackToIndex(t *testing.T) {
	var srv *httptest.Server
	var dir string
	var resp *http.Response
	var body string
	var path string

	var err error

	dir = t.TempDir()
	err = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	srv = newTestServer(t, Options{WebDir: dir})

	for _, path = range []string{"/", "/sessions/abc", "/index.html"} {
		resp, body = doRequest(t, http.MethodGet, srv.URL+path, nil)
		if resp.StatusCode != http.StatusOK || body != "<html>app</html>" {
			t.Fatalf("%s: status = %d body = %q", path, resp.StatusCode, body)
		}
	}

	resp, body = doRequest(t, http.MethodGet, srv.URL+"/app.js", nil)
	if resp.StatusCode != http.StatusOK || body != "console.log(1)" {
		t.Fatalf("/app.js: status = %d body = %q", resp.StatusCode, body)
	}

	resp, body = doRequest(t, http.MethodGet, srv.URL+"/api/unknown", nil)
	if resp.StatusCode != http.StatusNotFound || strings.Contains(body, "<html>") {
		t.Fatalf("/api/unknown: status = %d body = %q", resp.StatusCode, body)
	}

	resp, _ = doRequest(t, http.MethodPost, srv.URL+"/sessions/abc", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST fallback: status = %d, want 404", resp.StatusCode)
	}
}

func TestWebDirUnsetServesNothing(t *testing.T) {
	var srv *httptest.Server
	var resp *http.Response

	srv = newTestServer(t, Options{})

	resp, _ = doRequest(t, http.MethodGet, srv.URL+"/", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
