// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

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

func TestModeLookupDefaultsToDefault(t *testing.T) {
	setupTestFS(t)

	if ModeLookup("/home/user/project") != ModeDefault {
		t.Fatalf("lookup with no entries = %q, want %q", ModeLookup("/home/user/project"), ModeDefault)
	}
}

func TestModeLookupCoversSubdirectories(t *testing.T) {
	var err error

	setupTestFS(t)

	err = ModeUpsert("/home/user/project", ModeAutoPersist)
	if err != nil {
		t.Fatal(err)
	}

	if ModeLookup("/home/user/project/sub") != ModeAutoPersist {
		t.Fatalf("subdirectory lookup = %q, want %q", ModeLookup("/home/user/project/sub"), ModeAutoPersist)
	}
	if ModeLookup("/home/user/project2") != ModeDefault {
		t.Fatalf("sibling directory with a similar prefix leaked trust: %q", ModeLookup("/home/user/project2"))
	}
}

func TestModeLookupMostSpecificWins(t *testing.T) {
	var err error

	setupTestFS(t)

	err = ModeUpsert("/home/user", ModeAutoPersist)
	if err != nil {
		t.Fatal(err)
	}
	err = ModeUpsert("/home/user/scary", ModeDefault)
	if err != nil {
		t.Fatal(err)
	}

	if ModeLookup("/home/user/project") != ModeAutoPersist {
		t.Fatalf("ancestor-only lookup = %q, want %q", ModeLookup("/home/user/project"), ModeAutoPersist)
	}
	if ModeLookup("/home/user/scary/sub") != ModeDefault {
		t.Fatalf("more specific default entry lost to the auto_persist ancestor: %q", ModeLookup("/home/user/scary/sub"))
	}
}

func TestModeUpsertReplacesExistingEntry(t *testing.T) {
	var config *DirectoryConfig

	var err error

	setupTestFS(t)

	err = ModeUpsert("/home/user/project", ModeAutoPersist)
	if err != nil {
		t.Fatal(err)
	}
	err = ModeUpsert("/home/user/project", ModeFullAuto)
	if err != nil {
		t.Fatal(err)
	}

	config, err = ModeLoad()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Entries) != 1 {
		t.Fatalf("entries = %d, want 1 (upsert should replace, not duplicate)", len(config.Entries))
	}
	if config.Entries[0].Mode != ModeFullAuto {
		t.Fatalf("mode = %q, want %q", config.Entries[0].Mode, ModeFullAuto)
	}
}

func TestModeDefaultLocksDownASubdirectoryOfAFullAutoAncestor(t *testing.T) {
	var err error

	setupTestFS(t)

	err = ModeUpsert("/home/user", ModeFullAuto)
	if err != nil {
		t.Fatal(err)
	}
	err = ModeUpsert("/home/user/locked", ModeDefault)
	if err != nil {
		t.Fatal(err)
	}

	if ModeLookup("/home/user/locked") != ModeDefault {
		t.Fatalf("explicit default entry did not override the full_auto ancestor: %q", ModeLookup("/home/user/locked"))
	}
	if ModeLookup("/home/user/other") != ModeFullAuto {
		t.Fatalf("unrelated sibling lost the ancestor's full_auto mode: %q", ModeLookup("/home/user/other"))
	}
}

func TestModeEscapesAnchorForAbsoluteFilePath(t *testing.T) {
	if !ModeEscapesAnchor("/home/user/project", `{"path":"/etc/passwd","content":"x"}`) {
		t.Fatal("an absolute file path was not detected as an escape")
	}
	if !ModeEscapesAnchor("/home/user/project", `{"path":"../../etc/passwd"}`) {
		t.Fatal("a relative traversal out of the anchor was not detected as an escape")
	}
	if ModeEscapesAnchor("/home/user/project", `{"path":"sub/file.go"}`) {
		t.Fatal("a path inside the anchor was flagged as an escape")
	}
}

func TestIsIOTool(t *testing.T) {
	if !IsIOTool("file_read") || !IsIOTool("file_write") || !IsIOTool("file_edit") {
		t.Fatal("the three file tools must be IO tools")
	}
	if IsIOTool("bash_exec") || IsIOTool("session_send") || IsIOTool("agent_spawn") {
		t.Fatal("only the file tools are IO tools")
	}
}

func TestIsReadOnlyTool(t *testing.T) {
	if !IsReadOnlyTool("file_read") || !IsReadOnlyTool("browser_read") || !IsReadOnlyTool("browser_screenshot") {
		t.Fatal("file_read, browser_read, and browser_screenshot must be read-only")
	}
	if IsReadOnlyTool("file_write") || IsReadOnlyTool("file_edit") || IsReadOnlyTool("bash_exec") || IsReadOnlyTool("browser_navigate") {
		t.Fatal("a mutating tool was reported as read-only")
	}
}

func TestSessionModeOverride(t *testing.T) {
	var mode string
	var ok bool

	_, ok = SessionModeOverride("plan-override-s1")
	if ok {
		t.Fatal("a session with no override reported one")
	}

	SetSessionModeOverride("plan-override-s1", ModeAutoPersist)

	mode, ok = SessionModeOverride("plan-override-s1")
	if !ok || mode != ModeAutoPersist {
		t.Fatalf("override = (%q, %v), want (%q, true)", mode, ok, ModeAutoPersist)
	}

	ClearSessionModeOverride("plan-override-s1")

	_, ok = SessionModeOverride("plan-override-s1")
	if ok {
		t.Fatal("override survived ClearSessionModeOverride")
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
