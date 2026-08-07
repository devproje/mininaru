package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicReplacesContentAndKeepsMode(t *testing.T) {
	var dir string
	var path string
	var info os.FileInfo
	var buf []byte
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
