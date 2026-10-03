// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package client

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadPostsMultipart(t *testing.T) {
	var srv *httptest.Server
	var tmp string
	var gotPath string
	var gotAuth string
	var gotName string
	var id string

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fh multipart.File
		var hdr *multipart.FileHeader

		var err error

		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		fh, hdr, err = r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		fh.Close()
		gotName = hdr.Filename

		json.NewEncoder(w).Encode(map[string]string{"id": "att-99", "mime": "image/png"})
	}))
	defer srv.Close()

	tmp = filepath.Join(t.TempDir(), "shot.png")
	err = os.WriteFile(tmp, []byte("\x89PNGdata"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	id, err = Upload(srv.URL+"/api", "sk-1", "s1", tmp)
	if err != nil {
		t.Fatal(err)
	}

	if id != "att-99" {
		t.Fatalf("id = %q", id)
	}
	if gotPath != "/api/sessions/s1/attachments" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer sk-1" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotName != "shot.png" {
		t.Fatalf("filename = %q", gotName)
	}
}
