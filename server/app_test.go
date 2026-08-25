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

func TestNewAppServerWiresAPIRoutes(t *testing.T) {
	var app *AppServer
	var srv *httptest.Server
	var paths []string
	var path string
	var resp *http.Response

	var err error

	setupTestDB(t)
	app = NewAppServer("127.0.0.1", 0)
	srv = httptest.NewServer(app.WebServer.Handler)
	t.Cleanup(srv.Close)

	paths = []string{"/api/agents", "/api/providers", "/api/v1/models"}

	for _, path = range paths {
		resp, err = http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestNewAppServerWiresWebSocket(t *testing.T) {
	var app *AppServer
	var srv *httptest.Server
	var wsURL string
	var conn *websocket.Conn
	var frame struct {
		Type string `json:"type"`
	}

	var err error

	setupTestDB(t)
	app = NewAppServer("127.0.0.1", 0)
	srv = httptest.NewServer(app.WebServer.Handler)
	t.Cleanup(srv.Close)

	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
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
