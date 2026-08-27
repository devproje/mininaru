// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

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

const testAPIKey = "test-key"

func TestNewAppServerWiresAPIRoutes(t *testing.T) {
	var app *AppServer
	var srv *httptest.Server
	var paths []string
	var path string
	var req *http.Request
	var resp *http.Response

	var err error

	setupTestDB(t)
	app = NewAppServer("127.0.0.1", 0, testAPIKey)
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
	app = NewAppServer("127.0.0.1", 0, testAPIKey)
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
	app = NewAppServer("127.0.0.1", 0, testAPIKey)
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
