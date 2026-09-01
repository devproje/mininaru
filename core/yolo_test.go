// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/util"
)

func setupTestFS(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
}

func TestYoloLookupDefaultsToOff(t *testing.T) {
	setupTestFS(t)

	if YoloLookup("/home/user/project") != YoloOff {
		t.Fatalf("lookup with no entries = %q, want %q", YoloLookup("/home/user/project"), YoloOff)
	}
}

func TestYoloLookupCoversSubdirectories(t *testing.T) {
	var err error

	setupTestFS(t)

	err = YoloUpsert("/home/user/project", YoloPersist)
	if err != nil {
		t.Fatal(err)
	}

	if YoloLookup("/home/user/project/sub") != YoloPersist {
		t.Fatalf("subdirectory lookup = %q, want %q", YoloLookup("/home/user/project/sub"), YoloPersist)
	}
	if YoloLookup("/home/user/project2") != YoloOff {
		t.Fatalf("sibling directory with a similar prefix leaked trust: %q", YoloLookup("/home/user/project2"))
	}
}

func TestYoloLookupMostSpecificWins(t *testing.T) {
	var err error

	setupTestFS(t)

	err = YoloUpsert("/home/user", YoloPersist)
	if err != nil {
		t.Fatal(err)
	}
	err = YoloUpsert("/home/user/scary", YoloOff)
	if err != nil {
		t.Fatal(err)
	}

	if YoloLookup("/home/user/project") != YoloPersist {
		t.Fatalf("ancestor-only lookup = %q, want %q", YoloLookup("/home/user/project"), YoloPersist)
	}
	if YoloLookup("/home/user/scary/sub") != YoloOff {
		t.Fatalf("more specific off entry lost to the persist ancestor: %q", YoloLookup("/home/user/scary/sub"))
	}
}

func TestYoloUpsertReplacesExistingEntry(t *testing.T) {
	var config *DirectoryConfig

	var err error

	setupTestFS(t)

	err = YoloUpsert("/home/user/project", YoloPersist)
	if err != nil {
		t.Fatal(err)
	}
	err = YoloUpsert("/home/user/project", YoloOn)
	if err != nil {
		t.Fatal(err)
	}

	config, err = YoloLoad()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Entries) != 1 {
		t.Fatalf("entries = %d, want 1 (upsert should replace, not duplicate)", len(config.Entries))
	}
	if config.Entries[0].Mode != YoloOn {
		t.Fatalf("mode = %q, want %q", config.Entries[0].Mode, YoloOn)
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	var cases map[string]bool
	var addr string
	var want bool

	cases = map[string]bool{
		"127.0.0.1:54321": true,
		"[::1]:54321":     true,
		"localhost:54321": true,
		"203.0.113.5:443": false,
		"":                false,
	}

	for addr, want = range cases {
		if IsLoopbackAddr(addr) != want {
			t.Fatalf("IsLoopbackAddr(%q) = %v, want %v", addr, !want, want)
		}
	}
}

func TestResolveAnchorTrustsClientCwdOnlyWhenLoopback(t *testing.T) {
	var anchor string

	anchor = ResolveAnchor("127.0.0.1:1234", "/home/user/project")
	if anchor != "/home/user/project" {
		t.Fatalf("loopback anchor = %q, want the client cwd", anchor)
	}

	anchor = ResolveAnchor("203.0.113.5:1234", "/home/user/project")
	if anchor == "/home/user/project" {
		t.Fatal("a remote peer's claimed cwd was trusted as the anchor")
	}
}

func TestYoloOffLocksDownASubdirectoryOfAPersistedAncestor(t *testing.T) {
	var err error

	setupTestFS(t)

	err = YoloUpsert("/home/user", YoloOn)
	if err != nil {
		t.Fatal(err)
	}
	err = YoloUpsert("/home/user/locked", YoloOff)
	if err != nil {
		t.Fatal(err)
	}

	if YoloLookup("/home/user/locked") != YoloOff {
		t.Fatalf("explicit off entry did not override the on ancestor: %q", YoloLookup("/home/user/locked"))
	}
	if YoloLookup("/home/user/other") != YoloOn {
		t.Fatalf("unrelated sibling lost the ancestor's on mode: %q", YoloLookup("/home/user/other"))
	}
}

func TestAllowedAnchorConfinesRemotePeersToHome(t *testing.T) {
	var home string
	var anchor string

	var err error

	home, err = os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	anchor = allowedAnchor("127.0.0.1:1234", "/etc")
	if anchor != "/etc" {
		t.Fatalf("loopback anchor = %q, want the client cwd", anchor)
	}

	anchor = allowedAnchor("203.0.113.5:1234", "/etc")
	if anchor != "" {
		t.Fatalf("remote anchor = %q, want a refusal outside home", anchor)
	}

	anchor = allowedAnchor("203.0.113.5:1234", filepath.Join(home, "project"))
	if anchor != filepath.Join(home, "project") {
		t.Fatalf("remote anchor = %q, want a path under home", anchor)
	}

	anchor = allowedAnchor("203.0.113.5:1234", filepath.Join(home, "..", "..", "etc"))
	if anchor != "" {
		t.Fatalf("remote anchor = %q, want traversal out of home refused", anchor)
	}

	anchor = allowedAnchor("203.0.113.5:1234", "relative/path")
	if anchor != "" {
		t.Fatalf("remote anchor = %q, want a relative cwd refused", anchor)
	}
}
