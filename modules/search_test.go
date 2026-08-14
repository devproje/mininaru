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

func searchTree(t *testing.T) string {
	var root string
	var dir string
	var dirs []string

	var err error

	t.Helper()

	root = t.TempDir()
	dirs = []string{"cli", filepath.Join("core", "deep"), ".git", "node_modules"}

	for _, dir = range dirs {
		err = os.MkdirAll(filepath.Join(root, dir), 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(root, "cli", "root.go"), []byte("package cli\n\nvar needle = 1\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(root, "core", "deep", "agent.go"), []byte("package deep\n\nvar NEEDLE = 2\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(root, "readme.md"), []byte("needle in prose\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(root, ".git", "config"), []byte("needle\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(root, "node_modules", "index.go"), []byte("needle\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	return root
}

func TestGlobMatchesAcrossDirectories(t *testing.T) {
	var root string
	var result string

	var err error

	root = searchTree(t)

	result, err = Glob(root).Execute(context.Background(), `{"pattern":"**/*.go"}`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "main.go") || !strings.Contains(result, "cli/root.go") ||
		!strings.Contains(result, "core/deep/agent.go") {
		t.Fatalf("glob missed a file: %q", result)
	}
	if strings.Contains(result, "readme.md") {
		t.Fatalf("glob matched an unrelated file: %q", result)
	}
}

func TestGlobSkipsHiddenAndVendorDirectories(t *testing.T) {
	var root string
	var result string

	var err error

	root = searchTree(t)

	result, err = Glob(root).Execute(context.Background(), `{"pattern":"**/*"}`)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result, ".git") {
		t.Fatalf("glob walked into .git: %q", result)
	}
	if strings.Contains(result, "node_modules") {
		t.Fatalf("glob walked into node_modules: %q", result)
	}
}

func TestGlobHonoursMaxResults(t *testing.T) {
	var root string
	var result string

	var err error

	root = searchTree(t)

	result, err = Glob(root).Execute(context.Background(), `{"pattern":"**/*.go","max_results":1}`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "[truncated]") {
		t.Fatalf("glob did not report truncation: %q", result)
	}
	if len(strings.Split(result, "\n")) != 2 {
		t.Fatalf("glob returned more than one result: %q", result)
	}
}

func TestGrepReportsPathLineAndText(t *testing.T) {
	var root string
	var result string

	var err error

	root = searchTree(t)

	result, err = Grep(root).Execute(context.Background(), `{"pattern":"needle","glob":"**/*.go"}`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "cli/root.go:3:var needle = 1") {
		t.Fatalf("grep result = %q", result)
	}
	if strings.Contains(result, "readme.md") {
		t.Fatalf("grep ignored its glob filter: %q", result)
	}
	if strings.Contains(result, ".git") || strings.Contains(result, "node_modules") {
		t.Fatalf("grep walked a skipped directory: %q", result)
	}
}

func TestGrepIgnoresCaseOnRequest(t *testing.T) {
	var root string
	var result string

	var err error

	root = searchTree(t)

	result, err = Grep(root).Execute(context.Background(), `{"pattern":"needle","ignore_case":true,"glob":"**/*.go"}`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "core/deep/agent.go") {
		t.Fatalf("case insensitive grep missed NEEDLE: %q", result)
	}
}

func TestGrepRejectsInvalidPattern(t *testing.T) {
	var root string

	var err error

	root = searchTree(t)

	_, err = Grep(root).Execute(context.Background(), `{"pattern":"("}`)
	if err == nil {
		t.Fatal("grep accepted an invalid regular expression")
	}
	if !strings.Contains(err.Error(), "invalid pattern") {
		t.Fatalf("grep error = %v", err)
	}
}

func TestGrepSkipsBinaryFiles(t *testing.T) {
	var root string
	var result string

	var err error

	root = t.TempDir()

	err = os.WriteFile(filepath.Join(root, "blob.bin"), []byte("needle\x00needle"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(root, "plain.txt"), []byte("needle\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	result, err = Grep(root).Execute(context.Background(), `{"pattern":"needle"}`)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result, "blob.bin") {
		t.Fatalf("grep searched a binary file: %q", result)
	}
	if !strings.Contains(result, "plain.txt") {
		t.Fatalf("grep missed the text file: %q", result)
	}
}

func TestSearchDoesNotFollowSymlinks(t *testing.T) {
	var root string
	var outside string
	var globbed string
	var grepped string

	var err error

	root = t.TempDir()
	outside = t.TempDir()

	err = os.WriteFile(filepath.Join(outside, "secret.go"), []byte("needle\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(outside, filepath.Join(root, "escape"))
	if err != nil {
		t.Fatal(err)
	}

	globbed, err = Glob(root).Execute(context.Background(), `{"pattern":"**/*.go"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(globbed, "secret.go") {
		t.Fatalf("glob followed a symlink outside the root: %q", globbed)
	}

	grepped, err = Grep(root).Execute(context.Background(), `{"pattern":"needle"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(grepped, "secret.go") {
		t.Fatalf("grep followed a symlink outside the root: %q", grepped)
	}
}

func TestSearchRejectsPathOutsideRoot(t *testing.T) {
	var root string
	var outside string

	var err error

	root = t.TempDir()
	outside = t.TempDir()

	err = os.Symlink(outside, filepath.Join(root, "escape"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = Glob(root).Execute(context.Background(), `{"pattern":"**/*","path":"escape"}`)
	if err == nil {
		t.Fatal("glob accepted a path outside the root")
	}

	_, err = Grep(root).Execute(context.Background(), `{"pattern":"needle","path":"../"}`)
	if err == nil {
		t.Fatal("grep accepted a path outside the root")
	}
}
