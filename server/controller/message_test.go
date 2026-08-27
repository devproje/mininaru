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

func TestMessageCreateListReadUpdateDelete(t *testing.T) {
	var router *gin.Engine
	var sessionId string
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var created map[string]any
	var id string

	setupTestDB(t)
	router = newRouter()
	_, sessionId = createTestSession(t)

	body, _ = json.Marshal(map[string]string{"role": "user", "content": "hello"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/"+sessionId+"/messages", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &created)
	id = created["id"].(string)
	if created["status"] != "pending" {
		t.Fatalf("status = %v, want pending", created["status"])
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionId+"/messages", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list = %d messages, want 1", len(list))
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/messages/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("read status = %d, body = %s", w.Code, w.Body.String())
	}

	body, _ = json.Marshal(map[string]string{"status": "completed", "content": "hi there"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/messages/"+id, bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated["status"] != "completed" || updated["content"] != "hi there" {
		t.Fatalf("after update = %+v", updated)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/messages/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/messages/"+id, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("read after delete status = %d, want 404", w.Code)
	}
}

func TestMessageCreateRejectsUnknownSession(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"role": "user", "content": "hi"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/missing/messages", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestMessageReadNotFound(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	setupTestDB(t)
	router = newRouter()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/messages/missing", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
