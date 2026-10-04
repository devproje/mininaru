// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicReplacesContentAndKeepsMode(t *testing.T) {
	var dir string
	var path string
	var buf []byte
	var info os.FileInfo
	var leftovers []os.DirEntry

	var err error

	dir = t.TempDir()
	path = filepath.Join(dir, "config.json")

	err = WriteFileAtomic(path, []byte("first"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = WriteFileAtomic(path, []byte("second"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	buf, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "second" {
		t.Fatalf("content = %q, want the replacement", buf)
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}

	leftovers, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 1 {
		t.Fatalf("directory holds %d entries, want only the target file", len(leftovers))
	}
}

func TestSafeJoinRejectsEscapesUnlessUnrestricted(t *testing.T) {
	var root string
	var full string

	var err error

	root = t.TempDir()

	_, err = SafeJoin(root, "/etc/passwd", false)
	if err == nil {
		t.Fatal("an absolute path was accepted while restricted")
	}
	_, err = SafeJoin(root, "../../etc/passwd", false)
	if err == nil {
		t.Fatal("a relative escape was accepted while restricted")
	}

	full, err = SafeJoin(root, "/etc/passwd", true)
	if err != nil {
		t.Fatalf("an absolute path was rejected while unrestricted: %v", err)
	}
	if full != "/etc/passwd" {
		t.Fatalf("full = %q, want the absolute path unchanged", full)
	}
}

func TestInitFSKeepsDataDirectoryPrivate(t *testing.T) {
	var dir string
	var info os.FileInfo

	var err error

	dir = filepath.Join(t.TempDir(), "data")

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = InitFS(dir)
	if err != nil {
		t.Fatal(err)
	}

	info, err = os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("data directory mode = %v, want 0700 even when it already existed", info.Mode().Perm())
	}
}
