// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"testing"
)

func TestBootstrapSkippedForVersion(t *testing.T) {
	var dir string
	var entries []os.DirEntry

	var err error

	dir = t.TempDir()

	t.Setenv("NARU_PATH", dir+"/data")

	versionRef = true
	t.Cleanup(func() { versionRef = false })

	err = bootstrapExecute(root, nil)
	if err != nil {
		t.Fatalf("bootstrapExecute returned %v", err)
	}

	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("--version created %d entries under the data root, want 0", len(entries))
	}
}
