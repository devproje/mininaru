// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxUnit(t *testing.T) {
	var unit string
	var want string

	unit = linuxUnit("/opt/mininaru", "/var/lib/mininaru", "127.0.0.1", 9000)
	for _, want = range []string{
		"Environment=NARU_PATH=/var/lib/mininaru",
		"ExecStart=/opt/mininaru serve --host 127.0.0.1 --port 9000",
		"WantedBy=default.target",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
}

func TestDarwinPlist(t *testing.T) {
	var plist string
	var want string

	plist = darwinPlist("/opt/mininaru", "/Users/x/.mininaru", "127.0.0.1", 9000)
	for _, want = range []string{
		"<string>net.projecttl.mininaru</string>",
		"<string>/opt/mininaru</string>",
		"<string>/Users/x/.mininaru</string>",
		"<string>9000</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q:\n%s", want, plist)
		}
	}
}

func TestWindowsTaskAction(t *testing.T) {
	var got string
	var want string

	daemonHostRef, daemonPortRef = "127.0.0.1", 9000

	got = windowsTaskAction(`C:\Program Files\mininaru.exe`)
	want = `"C:\Program Files\mininaru.exe" serve --host 127.0.0.1 --port 9000`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPinNaruPath(t *testing.T) {
	var home string
	var rc string
	var body []byte

	var err error

	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	rc = filepath.Join(home, ".bashrc")

	err = os.WriteFile(rc, []byte("# existing\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	pinNaruPath("/data/dir")
	pinNaruPath("/data/dir") // idempotent

	body, err = os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), daemonEnvBegin) != 1 {
		t.Fatalf("env block not written exactly once:\n%s", body)
	}
	if !strings.Contains(string(body), `export NARU_PATH="/data/dir"`) {
		t.Fatalf("missing export:\n%s", body)
	}

	unpinNaruPath()

	body, err = os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# existing\n" {
		t.Fatalf("unpin did not restore original rc, got:\n%q", string(body))
	}
}
