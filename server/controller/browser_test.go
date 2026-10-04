// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func browserDo(t *testing.T, router *gin.Engine, method string, body any) *httptest.ResponseRecorder {
	var w *httptest.ResponseRecorder
	var req *http.Request
	var raw []byte

	t.Helper()

	if body != nil {
		raw, _ = json.Marshal(body)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(method, "/api/browser", bytes.NewReader(raw))
	router.ServeHTTP(w, req)

	return w
}

func TestBrowserReadDefaultsToAuto(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var resp browserConfigResponse

	setupTestDB(t)
	router = newRouter()

	w = browserDo(t, router, http.MethodGet, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Channel != "" {
		t.Fatalf("channel = %q, want empty before anything is configured", resp.Channel)
	}
}

func TestBrowserUpdateThenReadRoundtrips(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var resp browserConfigResponse

	setupTestDB(t)
	router = newRouter()

	w = browserDo(t, router, http.MethodPost, map[string]string{"channel": "edge"})
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", w.Code, w.Body.String())
	}

	w = browserDo(t, router, http.MethodGet, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("read status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Channel != "edge" {
		t.Fatalf("channel = %q, want edge", resp.Channel)
	}
}

func TestBrowserUpdateRejectsAnUnknownChannel(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder

	setupTestDB(t)
	router = newRouter()

	w = browserDo(t, router, http.MethodPost, map[string]string{"channel": "safari"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}
