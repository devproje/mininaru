// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileReadAndWriteStayUnderRoot(t *testing.T) {
	var root string
	var outside string
	var result string

	var err error

	root = t.TempDir()
	outside = t.TempDir()
	result, err = FileWrite(root).Execute(context.Background(), `{"path":"note.txt","content":"hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "5 bytes") {
		t.Fatalf("write result = %q", result)
	}

	result, err = FileRead(root).Execute(context.Background(), `{"path":"note.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Fatalf("read result = %q", result)
	}

	err = os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(outside, filepath.Join(root, "escape"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = FileRead(root).Execute(context.Background(), `{"path":"escape/secret.txt"}`)
	if err == nil {
		t.Fatal("file_read followed a symlink outside the root")
	}
	_, err = FileWrite(root).Execute(context.Background(), `{"path":"escape/new.txt","content":"bad"}`)
	if err == nil {
		t.Fatal("file_write followed a symlink outside the root")
	}
	err = os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "secret-link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = FileWrite(root).Execute(context.Background(), `{"path":"secret-link.txt","content":"bad"}`)
	if err == nil {
		t.Fatal("file_write followed a file symlink outside the root")
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
	result, err = FileRead(root).Execute(context.Background(), `{"path":"large.txt","max_chars":4}`)
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

	result, err = FileRead(root).Execute(context.Background(), `{"path":"lines.txt","offset":2,"limit":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "two\nthree" {
		t.Fatalf("ranged result = %q", result)
	}

	result, err = FileRead(root).Execute(context.Background(), `{"path":"lines.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "one\ntwo\nthree\nfour" {
		t.Fatalf("unranged result = %q", result)
	}

	result, err = FileRead(root).Execute(context.Background(), `{"path":"lines.txt","offset":99}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("out of range result = %q", result)
	}
}

func TestFileEditReplacesOneOccurrence(t *testing.T) {
	var root string
	var result string
	var buf []byte
	var info os.FileInfo

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "note.txt"), []byte("alpha beta gamma"), 0640)
	if err != nil {
		t.Fatal(err)
	}

	result, err = FileEdit(root).Execute(context.Background(), `{"path":"note.txt","old_string":"beta","new_string":"delta"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "replaced 1") {
		t.Fatalf("edit result = %q", result)
	}

	buf, err = os.ReadFile(filepath.Join(root, "note.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "alpha delta gamma" {
		t.Fatalf("edited content = %q", string(buf))
	}

	info, err = os.Stat(filepath.Join(root, "note.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Fatalf("edit changed the file mode to %v", info.Mode().Perm())
	}
}

func TestFileEditRejectsAmbiguousMatch(t *testing.T) {
	var root string
	var buf []byte

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "dup.txt"), []byte("x\nx\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = FileEdit(root).Execute(context.Background(), `{"path":"dup.txt","old_string":"x","new_string":"y"}`)
	if err == nil {
		t.Fatal("file_edit replaced an ambiguous match")
	}
	if !strings.Contains(err.Error(), "matches 2 times") {
		t.Fatalf("ambiguous error = %v", err)
	}

	buf, err = os.ReadFile(filepath.Join(root, "dup.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "x\nx\n" {
		t.Fatalf("rejected edit still wrote %q", string(buf))
	}
}

func TestFileEditReplacesAll(t *testing.T) {
	var root string
	var result string
	var buf []byte

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "dup.txt"), []byte("x\nx\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	result, err = FileEdit(root).Execute(context.Background(), `{"path":"dup.txt","old_string":"x","new_string":"y","replace_all":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "replaced 2") {
		t.Fatalf("replace_all result = %q", result)
	}

	buf, err = os.ReadFile(filepath.Join(root, "dup.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "y\ny\n" {
		t.Fatalf("replace_all content = %q", string(buf))
	}
}

func TestFileEditRejectsMissingAndIdenticalStrings(t *testing.T) {
	var root string

	var err error

	root = t.TempDir()
	err = os.WriteFile(filepath.Join(root, "note.txt"), []byte("alpha"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = FileEdit(root).Execute(context.Background(), `{"path":"note.txt","old_string":"absent","new_string":"x"}`)
	if err == nil {
		t.Fatal("file_edit accepted a string that is not in the file")
	}

	_, err = FileEdit(root).Execute(context.Background(), `{"path":"note.txt","old_string":"alpha","new_string":"alpha"}`)
	if err == nil {
		t.Fatal("file_edit accepted an edit that changes nothing")
	}
}

func TestFileEditStaysUnderRoot(t *testing.T) {
	var root string
	var outside string
	var buf []byte

	var err error

	root = t.TempDir()
	outside = t.TempDir()

	err = os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(outside, filepath.Join(root, "escape"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = FileEdit(root).Execute(context.Background(), `{"path":"escape/secret.txt","old_string":"secret","new_string":"leaked"}`)
	if err == nil {
		t.Fatal("file_edit followed a symlink outside the root")
	}

	buf, err = os.ReadFile(filepath.Join(outside, "secret.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "secret" {
		t.Fatalf("file outside the root was rewritten to %q", string(buf))
	}
}
