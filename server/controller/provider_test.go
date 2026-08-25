// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
)

func TestProviderCreateMasksApiKey(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var body []byte
	var created map[string]any
	var stored *core.Provider

	var err error

	setupTestDB(t)
	router = newRouter()

	body, _ = json.Marshal(map[string]string{"name": "one", "api_key": "sk-superlongsecretkey1234"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/providers", bytes.NewReader(body))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &created)
	if created["api_key"] == "sk-superlongsecretkey1234" {
		t.Fatal("response api_key was not masked")
	}
	if !strings.Contains(created["api_key"].(string), "...") {
		t.Fatalf("api_key = %q, expected a masked form", created["api_key"])
	}

	stored, err = core.ProviderRead(created["id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if stored.ApiKey != "sk-superlongsecretkey1234" {
		t.Fatalf("stored api_key = %q, want the real unmasked value", stored.ApiKey)
	}
}

func TestProviderListMasksApiKey(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	var err error

	setupTestDB(t)
	router = newRouter()

	err = core.ProviderCreate(&core.Provider{Id: "p1", Name: "one", ApiKey: "sk-superlongsecretkey1234"})
	if err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", w.Code, w.Body.String())
	}

	var list []map[string]any
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list = %d providers, want 1", len(list))
	}
	if list[0]["api_key"] == "sk-superlongsecretkey1234" {
		t.Fatal("list response api_key was not masked")
	}
}

func TestProviderActivateSwitchesActiveProvider(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	var err error

	setupTestDB(t)
	router = newRouter()

	err = core.ProviderCreate(&core.Provider{Id: "p1", Name: "one"})
	if err != nil {
		t.Fatal(err)
	}
	err = core.ProviderCreate(&core.Provider{Id: "p2", Name: "two"})
	if err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/providers/p1/activate", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("activate p1 status = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/providers/p2/activate", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("activate p2 status = %d, body = %s", w.Code, w.Body.String())
	}

	var p1, p2 *core.Provider

	p1, err = core.ProviderRead("p1")
	if err != nil {
		t.Fatal(err)
	}
	p2, err = core.ProviderRead("p2")
	if err != nil {
		t.Fatal(err)
	}

	if p1.Active {
		t.Fatal("p1 should no longer be active after p2 was activated")
	}
	if !p2.Active {
		t.Fatal("p2 should be active")
	}
}

func TestProviderReadNotFound(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	setupTestDB(t)
	router = newRouter()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/providers/missing", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestProviderDelete(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	var err error

	setupTestDB(t)
	router = newRouter()

	err = core.ProviderCreate(&core.Provider{Id: "p1", Name: "one"})
	if err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/providers/p1", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}

	_, err = core.ProviderRead("p1")
	if err == nil {
		t.Fatal("expected an error reading a deleted provider")
	}
}
