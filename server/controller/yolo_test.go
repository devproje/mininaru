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

func TestYoloSetTrustsCwdOnlyWhenLoopback(t *testing.T) {
	var router *gin.Engine
	var body []byte
	var w *httptest.ResponseRecorder
	var req *http.Request
	var resp map[string]any

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"mode": "persist", "cwd": "/home/user/project"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/yolo", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["root"] != "/home/user/project" {
		t.Fatalf("root = %v, want the loopback client's cwd", resp["root"])
	}
	if core.YoloLookup("/home/user/project") != core.YoloPersist {
		t.Fatalf("YoloLookup after set = %q, want %q", core.YoloLookup("/home/user/project"), core.YoloPersist)
	}
}

func TestYoloSetIgnoresCwdFromANonLoopbackPeer(t *testing.T) {
	var router *gin.Engine
	var body []byte
	var w *httptest.ResponseRecorder
	var req *http.Request
	var resp map[string]any

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"mode": "on", "cwd": "/home/user/project"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/yolo", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.5:54321"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["root"] == "/home/user/project" {
		t.Fatal("a remote peer's claimed cwd was trusted as the anchor")
	}
}

func TestYoloGetReturnsTheUpsertedMode(t *testing.T) {
	var router *gin.Engine
	var body []byte
	var w *httptest.ResponseRecorder
	var req *http.Request
	var resp map[string]any

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"mode": "persist", "cwd": "/home/user/project"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/yolo", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set status = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/yolo?cwd=/home/user/project/sub", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["mode"] != "persist" {
		t.Fatalf("mode = %v, want persist (inherited from the covering entry)", resp["mode"])
	}
}

func TestYoloGetDefaultsToOff(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var resp map[string]any

	setupTestDB(t)
	router = newRouter()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/yolo?cwd=/nowhere", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["mode"] != "off" {
		t.Fatalf("mode = %v, want off", resp["mode"])
	}
}

func TestYoloSetRejectsUnknownMode(t *testing.T) {
	var router *gin.Engine
	var body []byte
	var w *httptest.ResponseRecorder
	var req *http.Request

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"mode": "yolo"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/yolo", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}
