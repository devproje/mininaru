// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"os"
	"testing"
)

func TestAPIKeyGeneratesAndPersists(t *testing.T) {
	var first string
	var second string
	var info os.FileInfo

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	first, err = APIKey()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" {
		t.Fatal("expected a non-empty generated key")
	}

	second, err = APIKey()
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("second call returned %q, want the same key %q", second, first)
	}

	info, err = os.Stat(Path(apiKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("key file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestAPIKeyDiffersAcrossDataDirectories(t *testing.T) {
	var first string
	var second string

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	first, err = APIKey()
	if err != nil {
		t.Fatal(err)
	}

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	second, err = APIKey()
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal("expected a fresh data directory to generate a different key")
	}
}
