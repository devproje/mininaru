// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/gorilla/websocket"
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

func TestTurnSendsAndClearsPendingImages(t *testing.T) {
	var srv *httptest.Server
	var conn *websocket.Conn
	var sh Shell
	var got Frame
	var done chan struct{}

	var err error

	done = make(chan struct{})

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var up websocket.Upgrader
		var server *websocket.Conn
		var frame Frame

		var err error

		defer close(done)

		server, err = up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer server.Close()

		for {
			frame = Frame{}

			err = server.ReadJSON(&frame)
			if err != nil {
				t.Error(err)
				return
			}

			if frame.Type == "attach" {
				continue
			}

			got = frame
			break
		}

		server.WriteJSON(Reply{Type: "done", SessionId: "s1"})
	}))
	defer srv.Close()

	conn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	sh = Shell{conn: conn, session: &core.Session{Id: "s1"}, cwd: "/tmp", pending: []string{"att-1", "att-2"}}
	sh.frames = Pump(conn)

	err = conn.WriteJSON(Frame{Type: "attach", SessionId: "s1"})
	if err != nil {
		t.Fatal(err)
	}

	err = sh.turn("look at this")
	if err != nil {
		t.Fatal(err)
	}

	<-done

	if got.Content != "look at this" || len(got.Images) != 2 || got.Images[0] != "att-1" {
		t.Fatalf("frame = %+v", got)
	}

	if sh.pending != nil {
		t.Fatalf("pending not cleared: %v", sh.pending)
	}
}
