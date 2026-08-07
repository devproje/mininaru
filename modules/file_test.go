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
