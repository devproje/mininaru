// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readBeforeModify(t *testing.T, root, path string) {
	var err error

	t.Helper()

	_, err = Read(root).Execute(context.Background(), `{"path":"`+path+`"}`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileReadAndWriteRoundtrip(t *testing.T) {
	var root string
	var result string

	var err error

	root = t.TempDir()

	result, err = Write(root).Execute(context.Background(), `{"path":"note.txt","content":"hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "--- a/note.txt") || !strings.Contains(result, "+hello") {
		t.Fatalf("write result = %q", result)
	}

	result, err = Read(root).Execute(context.Background(), `{"path":"note.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Fatalf("read result = %q", result)
	}
}

func TestFileReadRejectsPathEscape(t *testing.T) {
	var root string

	var err error

	root = t.TempDir()

	_, err = Read(root).Execute(context.Background(), `{"path":"../secret.txt"}`)
	if err == nil {
		t.Fatal("file_read accepted a path escaping the root")
	}
	_, err = Write(root).Execute(context.Background(), `{"path":"../secret.txt","content":"bad"}`)
	if err == nil {
		t.Fatal("file_write accepted a path escaping the root")
	}
}

func TestFileReadTruncatesOutput(t *testing.T) {
	var root string
	var result string

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "large.txt"), []byte("1234567890"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	result, err = Read(root).Execute(context.Background(), `{"path":"large.txt","max_chars":4}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "1234\n[truncated]" {
		t.Fatalf("truncated result = %q", result)
	}
}

func TestFileReadSelectsLineRange(t *testing.T) {
	var root string
	var result string

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "lines.txt"), []byte("one\ntwo\nthree\nfour"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	result, err = Read(root).Execute(context.Background(), `{"path":"lines.txt","offset":2,"limit":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "two\nthree" {
		t.Fatalf("ranged result = %q", result)
	}
}

func TestFileEditReplacesOneOccurrence(t *testing.T) {
	var root string
	var result string
	var buf []byte

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "note.txt"), []byte("alpha beta gamma"), 0640)
	if err != nil {
		t.Fatal(err)
	}
	readBeforeModify(t, root, "note.txt")

	result, err = Edit(root).Execute(context.Background(), `{"path":"note.txt","old_string":"beta","new_string":"delta"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "-alpha beta gamma") || !strings.Contains(result, "+alpha delta gamma") {
		t.Fatalf("edit result = %q", result)
	}

	buf, err = os.ReadFile(filepath.Join(root, "note.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "alpha delta gamma" {
		t.Fatalf("edited content = %q", string(buf))
	}
}

func TestFileEditRejectsAmbiguousMatch(t *testing.T) {
	var root string

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "dup.txt"), []byte("x\nx\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	readBeforeModify(t, root, "dup.txt")

	_, err = Edit(root).Execute(context.Background(), `{"path":"dup.txt","old_string":"x","new_string":"y"}`)
	if err == nil || !strings.Contains(err.Error(), "matches 2 times") {
		t.Fatalf("ambiguous error = %v", err)
	}
}

func TestFileEditReplacesAll(t *testing.T) {
	var root string
	var buf []byte

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "dup.txt"), []byte("x\nx\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	readBeforeModify(t, root, "dup.txt")

	_, err = Edit(root).Execute(context.Background(), `{"path":"dup.txt","old_string":"x","new_string":"y","replace_all":true}`)
	if err != nil {
		t.Fatal(err)
	}

	buf, err = os.ReadFile(filepath.Join(root, "dup.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "y\ny\n" {
		t.Fatalf("replace_all content = %q", string(buf))
	}
}

func TestFileModifyRequiresFreshRead(t *testing.T) {
	var root string
	var path string
	var buf []byte

	var err error

	root = t.TempDir()
	path = filepath.Join(root, "note.txt")
	err = os.WriteFile(path, []byte("one"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Edit(root).Execute(context.Background(), `{"path":"note.txt","old_string":"one","new_string":"two"}`)
	if err == nil || !strings.Contains(err.Error(), "file_read") {
		t.Fatalf("edit without read error = %v", err)
	}

	readBeforeModify(t, root, "note.txt")
	err = os.WriteFile(path, []byte("external"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Write(root).Execute(context.Background(), `{"path":"note.txt","content":"overwrite"}`)
	if err == nil || !strings.Contains(err.Error(), "changed since file_read") {
		t.Fatalf("stale write error = %v", err)
	}

	buf, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "external" {
		t.Fatalf("stale write changed file to %q", string(buf))
	}
}

func TestFileReadRejectsBinaryContent(t *testing.T) {
	var root string

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "binary.dat"), []byte{'a', 0, 'b'}, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read(root).Execute(context.Background(), `{"path":"binary.dat"}`)
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Fatalf("binary read error = %v", err)
	}
}
