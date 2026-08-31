// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func memReq(t *testing.T, router *gin.Engine, method string, path string, body any) *httptest.ResponseRecorder {
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

func TestMemoryEndpointRoundTrip(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var list struct {
		Index string   `json:"index"`
		Files []string `json:"files"`
	}
	var read struct {
		Content string `json:"content"`
	}

	setupTestDB(t)
	router = newRouter()
	createTestAgent(t)

	w = memReq(t, router, http.MethodPut, "/api/agents/a1/memory/likes-pnpm.md", map[string]string{
		"description": "prefers pnpm", "type": "feedback", "content": "# note\nalways pnpm",
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("put = %d: %s", w.Code, w.Body.String())
	}

	w = memReq(t, router, http.MethodGet, "/api/agents/a1/memory", nil)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Files) != 1 || list.Files[0] != "likes-pnpm.md" {
		t.Fatalf("list = %+v", list)
	}

	w = memReq(t, router, http.MethodGet, "/api/agents/a1/memory/likes-pnpm.md", nil)
	json.Unmarshal(w.Body.Bytes(), &read)
	if read.Content == "" || !bytes.Contains([]byte(read.Content), []byte("always pnpm")) {
		t.Fatalf("read = %q", read.Content)
	}

	w = memReq(t, router, http.MethodDelete, "/api/agents/a1/memory/likes-pnpm.md", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", w.Code)
	}

	w = memReq(t, router, http.MethodGet, "/api/agents/a1/memory/likes-pnpm.md", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("read after delete = %d, want 404", w.Code)
	}
}

func TestMemoryEndpointUnknownAgent(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder

	setupTestDB(t)
	router = newRouter()

	w = memReq(t, router, http.MethodGet, "/api/agents/nope/memory", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown agent = %d, want 404", w.Code)
	}
}

func TestMemoryEndpointRejectsBadBody(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder

	setupTestDB(t)
	router = newRouter()
	createTestAgent(t)

	w = memReq(t, router, http.MethodPut, "/api/agents/a1/memory/x.md", map[string]string{"description": "d", "type": "bogus", "content": "c"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad type = %d, want 400", w.Code)
	}
}
