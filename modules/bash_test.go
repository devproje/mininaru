package modules

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashExecUsesRoot(t *testing.T) {
	var root string
	var result string
	var err error

	root = t.TempDir()
	result, err = BashExec(root).Execute(context.Background(), `{"command":"pwd"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != filepath.Clean(root) {
		t.Fatalf("pwd = %q, want %q", result, root)
	}
}
