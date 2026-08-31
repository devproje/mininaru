// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

func writeTestSkill(t *testing.T, name string, description string) {
	var dir string

	var err error

	t.Helper()

	dir = util.Path(filepath.Join("skills", name))

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(dir, "SKILL.md"),
		[]byte("---\nname: "+name+"\ndescription: "+description+"\n---\nbody of "+name+"\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSkillListAndRead(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var list []skillSummary
	var one map[string]string

	setupTestDB(t)
	router = newRouter()
	writeTestSkill(t, "deploy", "how to deploy")

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/skill", nil)
	router.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "deploy" || list[0].Description != "how to deploy" {
		t.Fatalf("list = %+v", list)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/skill/deploy", nil)
	router.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &one)
	if one["name"] != "deploy" || one["text"] == "" {
		t.Fatalf("read = %+v", one)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/skill/missing", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown skill = %d, want 404", w.Code)
	}
}

func TestSkillUsesEmpty(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	setupTestDB(t)
	router = newRouter()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/skill/uses", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("uses = %d: %s", w.Code, w.Body.String())
	}
}
