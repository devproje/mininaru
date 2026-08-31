// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\nfake image body")

func uploadAttachment(t *testing.T, router *gin.Engine, sessionId string, filename string, data []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	var writer *multipart.Writer
	var part io.Writer
	var w *httptest.ResponseRecorder
	var req *http.Request

	var err error

	t.Helper()

	writer = multipart.NewWriter(&buf)
	part, err = writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(data)
	writer.Close()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/"+sessionId+"/attachments", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	return w
}

func TestAttachmentUploadAndDownload(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request
	var created map[string]any

	setupTestDB(t)
	router = newRouter()
	createTestSession(t)

	w = uploadAttachment(t, router, "s1", "shot.png", pngBytes)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload = %d: %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &created)
	if created["mime"] != "image/png" || created["id"] == "" {
		t.Fatalf("created = %+v", created)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/attachments/"+created["id"].(string), nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("download = %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatalf("download body mismatch")
	}
}

func TestAttachmentUploadRejectsNonImage(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder

	setupTestDB(t)
	router = newRouter()
	createTestSession(t)

	w = uploadAttachment(t, router, "s1", "notes.txt", []byte("just some plain text, definitely not an image"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-image upload = %d, want 400", w.Code)
	}
}

func TestAttachmentUploadUnknownSession(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder

	setupTestDB(t)
	router = newRouter()

	w = uploadAttachment(t, router, "nope", "shot.png", pngBytes)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown session = %d, want 404", w.Code)
	}
}

func TestAttachmentDownloadUnknown(t *testing.T) {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var req *http.Request

	setupTestDB(t)
	router = newRouter()

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/attachments/missing", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown attachment = %d, want 404", w.Code)
	}
}
