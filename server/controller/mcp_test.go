// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/modules/mcp"
	"github.com/gin-gonic/gin"
)

func mcpDo(t *testing.T, router *gin.Engine, method string, path string, body any) *httptest.ResponseRecorder {
	var w *httptest.ResponseRecorder
	var req *http.Request
	var raw []byte

	t.Helper()

	if body != nil {
		raw, _ = json.Marshal(body)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(method, path, bytes.NewReader(raw))
	router.ServeHTTP(w, req)

	return w
}

func TestMcpCrudRoundTrips(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var list []mcpEntry
	var one mcpEntry
	var disabled bool

	setupTestDB(t)
	router = newRouter()

	disabled = false

	w = mcpDo(t, router, http.MethodPost, "/api/mcp", mcp.Server{
		Name: "files", Transport: "stdio", Command: "true", Enabled: &disabled,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", w.Code, w.Body.String())
	}

	w = mcpDo(t, router, http.MethodPost, "/api/mcp", mcp.Server{Name: "files", Transport: "stdio", Command: "true"})
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate create = %d, want 409", w.Code)
	}

	w = mcpDo(t, router, http.MethodGet, "/api/mcp", nil)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Server.Name != "files" {
		t.Fatalf("list = %+v", list)
	}

	w = mcpDo(t, router, http.MethodPost, "/api/mcp/files/enable", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("enable = %d: %s", w.Code, w.Body.String())
	}

	w = mcpDo(t, router, http.MethodGet, "/api/mcp/files", nil)
	json.Unmarshal(w.Body.Bytes(), &one)
	if one.Server.Enabled == nil || !*one.Server.Enabled {
		t.Fatalf("enable did not stick: %+v", one.Server)
	}

	w = mcpDo(t, router, http.MethodDelete, "/api/mcp/files", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", w.Code)
	}

	w = mcpDo(t, router, http.MethodGet, "/api/mcp/files", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("read after delete = %d, want 404", w.Code)
	}
}

func TestMcpCreateValidates(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder

	setupTestDB(t)
	router = newRouter()

	w = mcpDo(t, router, http.MethodPost, "/api/mcp", mcp.Server{Name: "bad name", Transport: "stdio", Command: "true"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid name = %d, want 400", w.Code)
	}

	w = mcpDo(t, router, http.MethodPost, "/api/mcp", mcp.Server{Name: "nocmd", Transport: "stdio"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing command = %d, want 400", w.Code)
	}
}
