// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxUnit(t *testing.T) {
	var unit string
	var want string

	unit = linuxUnit("/opt/mininaru", "/var/lib/mininaru", []string{"serve", "--host", "127.0.0.1", "--port", "9000"})
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

	plist = darwinPlist("/opt/mininaru", "/Users/x/.mininaru", []string{"serve", "--host", "127.0.0.1", "--port", "9000"})
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

	got = windowsTaskAction(`C:\Program Files\mininaru.exe`, []string{"serve", "--host", "127.0.0.1", "--port", "9000"})
	want = `"C:\Program Files\mininaru.exe" serve --host 127.0.0.1 --port 9000`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDaemonExecArgs(t *testing.T) {
	var args []string
	var got string
	var want string

	daemonHostRef, daemonPortRef = "127.0.0.1", 9000
	daemonCorsOriginsRef = []string{"http://localhost:5173", "http://localhost:3000"}
	daemonWebDirRef = "/opt/mininaru/web"

	args = daemonExecArgs()
	got = strings.Join(args, " ")
	want = "serve --host 127.0.0.1 --port 9000 --cors-origin http://localhost:5173 --cors-origin http://localhost:3000 --web-dir /opt/mininaru/web"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	daemonCorsOriginsRef = nil
	daemonWebDirRef = ""
}

func TestQuoteArgWithSpaces(t *testing.T) {
	if quoteArg("/opt/my dir") != "'/opt/my dir'" {
		t.Fatalf("quoteArg did not quote a path with spaces: %q", quoteArg("/opt/my dir"))
	}
	if quoteArg("/opt/mininaru") != "/opt/mininaru" {
		t.Fatalf("quoteArg quoted a path without spaces: %q", quoteArg("/opt/mininaru"))
	}
	if winQuoteArg(`C:\my dir`) != `"C:\my dir"` {
		t.Fatalf("winQuoteArg did not quote a path with spaces: %q", winQuoteArg(`C:\my dir`))
	}
}

func TestShellTokenize(t *testing.T) {
	var got []string
	var want []string
	var i int

	got = shellTokenize(`/opt/mininaru serve --host 127.0.0.1 --port 9000 --web-dir '/opt/my dir' --cors-origin http://localhost:5173`)
	want = []string{"/opt/mininaru", "serve", "--host", "127.0.0.1", "--port", "9000", "--web-dir", "/opt/my dir", "--cors-origin", "http://localhost:5173"}

	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(got), len(want), got)
	}
	for i = range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestParseServeArgs(t *testing.T) {
	var preset daemonPreset

	preset = parseServeArgs([]string{"/opt/mininaru", "serve", "--host", "127.0.0.1", "--port", "9000",
		"--cors-origin", "http://localhost:5173", "--cors-origin", "http://localhost:3000", "--web-dir", "/opt/web"})

	if preset.Host != "127.0.0.1" || preset.Port != 9000 || preset.WebDir != "/opt/web" {
		t.Fatalf("unexpected preset: %+v", preset)
	}
	if len(preset.CorsOrigins) != 2 || preset.CorsOrigins[0] != "http://localhost:5173" || preset.CorsOrigins[1] != "http://localhost:3000" {
		t.Fatalf("unexpected cors origins: %+v", preset.CorsOrigins)
	}
}

func TestLinuxExistingConfig(t *testing.T) {
	var home string
	var unitPath string
	var preset daemonPreset
	var existed bool

	var err error

	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	preset, existed, err = linuxExistingConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		t.Fatalf("expected no existing unit, got %+v", preset)
	}

	unitPath, err = linuxUnitPath()
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(filepath.Dir(unitPath), 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(unitPath, []byte(linuxUnit("/opt/mininaru", "/var/lib/mininaru",
		[]string{"serve", "--host", "127.0.0.1", "--port", "9000", "--web-dir", "/opt/web"})), 0600)
	if err != nil {
		t.Fatal(err)
	}

	preset, existed, err = linuxExistingConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatal("expected the existing unit to be found")
	}
	if preset.Host != "127.0.0.1" || preset.Port != 9000 || preset.WebDir != "/opt/web" {
		t.Fatalf("unexpected preset: %+v", preset)
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
