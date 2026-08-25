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

func TestSessionCreateReadListUpdateDelete(t *testing.T) {
	var router *gin.Engine
	var agentId string
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var created map[string]any
	var id string

	setupTestDB(t)
	router = newRouter()
	agentId = createTestAgent(t)

	body, _ = json.Marshal(map[string]string{"agent_id": agentId, "name": "first"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &created)
	id = created["id"].(string)
	if created["created_at"] == "" {
		t.Fatal("created session has no created_at")
	}
	if created["name"] != "first" {
		t.Fatalf("created name = %v, want first", created["name"])
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/sessions/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("read status = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/sessions?agent_id="+agentId, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list = %d sessions, want 1", len(list))
	}

	body, _ = json.Marshal(map[string]string{"name": "renamed"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/sessions/"+id, bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated["name"] != "renamed" {
		t.Fatalf("name after update = %v, want renamed", updated["name"])
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/sessions/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/sessions/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("read after delete status = %d, want 404", w.Code)
	}
}

func TestSessionListRequiresAgentId(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	setupTestDB(t)
	router = newRouter()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSessionCreateRejectsUnknownAgent(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"agent_id": "missing"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestSessionReadNotFound(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	setupTestDB(t)
	router = newRouter()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/sessions/missing", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
