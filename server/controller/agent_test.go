// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
)

func TestAgentCreateAppliesSchemaDefaults(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var created map[string]any

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"name": "naru", "model": "gpt-4o-mini"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &created)
	if created["thinking_level"] != "medium" {
		t.Fatalf("thinking_level = %v, want the schema default medium", created["thinking_level"])
	}
	if created["max_context"].(float64) != 24000 {
		t.Fatalf("max_context = %v, want the schema default 24000", created["max_context"])
	}
}

func TestAgentReadListUpdateDelete(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var id string

	setupTestDB(t)
	router = newRouter()
	id = createTestAgent(t)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/agents/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("read status = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list = %d agents, want 1", len(list))
	}

	body, _ = json.Marshal(map[string]string{"soul": "be terse"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/agents/"+id, bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated["soul"] != "be terse" {
		t.Fatalf("soul after update = %v, want 'be terse'", updated["soul"])
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/agents/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/agents/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("read after delete status = %d, want 404", w.Code)
	}
}

func TestAgentDeleteCascadesSessions(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var agentId string

	var err error

	setupTestDB(t)
	router = newRouter()
	agentId, _ = createTestSession(t)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/agents/"+agentId, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}

	_, err = core.SessionRead("s1")
	if err == nil {
		t.Fatal("expected the agent's session to be gone after the agent was deleted")
	}
}

func TestAgentCreateRequiresFields(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"name": "naru"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}
