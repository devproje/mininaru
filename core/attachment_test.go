// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
	"github.com/openai/openai-go"
)

func writeTestImage(t *testing.T, id string) string {
	var path string

	var err error

	t.Helper()

	path = util.Path(filepath.Join("attachments", id))

	err = os.MkdirAll(filepath.Dir(path), 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, []byte("\x89PNG\r\n\x1a\nfake"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func TestAttachmentCreateValidates(t *testing.T) {
	var err error

	setupTestDB(t)
	setupTestSession(t)

	err = AttachmentCreate(&Attachment{Id: "x", SessionId: "s1", Mime: "text/plain", Path: "/tmp/x"})
	if err == nil {
		t.Fatal("non-image mime should be rejected")
	}

	err = AttachmentCreate(&Attachment{Id: "x", Mime: "image/png", Path: "/tmp/x"})
	if err == nil {
		t.Fatal("missing session_id should be rejected")
	}
}

func TestAttachmentBindAndImages(t *testing.T) {
	var images []string
	var path string

	var err error

	setupTestDB(t)
	setupTestSession(t)

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "look"})
	if err != nil {
		t.Fatal(err)
	}

	path = writeTestImage(t, "att1")

	err = AttachmentCreate(&Attachment{Id: "att1", SessionId: "s1", Mime: "image/png", Bytes: 12, Path: path})
	if err != nil {
		t.Fatal(err)
	}

	images, err = messageImages("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 0 {
		t.Fatalf("unbound attachment should not surface: %v", images)
	}

	err = AttachmentBindMessage("s1", "m1", []string{"att1"})
	if err != nil {
		t.Fatal(err)
	}

	images, err = messageImages("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || !strings.HasPrefix(images[0], "data:image/png;base64,") {
		t.Fatalf("bound image = %v", images)
	}

	err = AttachmentBindMessage("other-session", "m1", []string{"att1"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAttachmentDeleteRemovesFile(t *testing.T) {
	var path string

	var err error

	setupTestDB(t)
	setupTestSession(t)

	path = writeTestImage(t, "att2")

	err = AttachmentCreate(&Attachment{Id: "att2", SessionId: "s1", Mime: "image/png", Path: path})
	if err != nil {
		t.Fatal(err)
	}

	err = AttachmentDelete("att2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatalf("file should be gone, stat err = %v", err)
	}
}

func TestHistoryUnionEmitsImageParts(t *testing.T) {
	var history []*Message
	var union []openai.ChatCompletionMessageParamUnion
	var parts []openai.ChatCompletionContentPartUnionParam
	var path string

	var err error

	setupTestDB(t)
	setupTestSession(t)

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "what is this", Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}

	path = writeTestImage(t, "att-h")

	err = AttachmentCreate(&Attachment{Id: "att-h", SessionId: "s1", Mime: "image/png", Path: path})
	if err != nil {
		t.Fatal(err)
	}

	err = AttachmentBindMessage("s1", "m1", []string{"att-h"})
	if err != nil {
		t.Fatal(err)
	}

	history, err = MessageList("s1")
	if err != nil {
		t.Fatal(err)
	}

	union, _, err = historyUnion(history)
	if err != nil {
		t.Fatal(err)
	}

	if len(union) == 0 || union[0].OfUser == nil {
		t.Fatalf("first message is not a user message: %+v", union)
	}

	parts = union[0].OfUser.Content.OfArrayOfContentParts
	if len(parts) != 2 || parts[1].GetImageURL() == nil {
		t.Fatalf("want text + image parts, got %+v", parts)
	}
}

func TestAttachmentCascadesWithSession(t *testing.T) {
	var count int
	var path string

	var err error

	setupTestDB(t)
	setupTestSession(t)

	path = writeTestImage(t, "att3")

	err = AttachmentCreate(&Attachment{Id: "att3", SessionId: "s1", Mime: "image/png", Path: path})
	if err != nil {
		t.Fatal(err)
	}

	err = SessionDelete("s1")
	if err != nil {
		t.Fatal(err)
	}

	err = util.DB.QueryRow("SELECT COUNT(*) FROM attachments WHERE id = ?;", "att3").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("attachment row should cascade-delete with its session")
	}
}
