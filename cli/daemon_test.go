// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDaemonEnvValid(t *testing.T) {
	var path string

	var err error

	path = filepath.Join(t.TempDir(), "env")
	if err = os.WriteFile(path, []byte("MININARU_API_KEY=<REDACTED>\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = daemonEnvValid(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err = daemonEnvValid(path); err == nil {
		t.Fatal("world-readable environment file was accepted")
	}
}

func TestDaemonUnit(t *testing.T) {
	var unit string
	var expected string

	unit = daemonUnit("/opt/mininaru", "/var/lib/mininaru", "/srv/project", "/etc/mininaru/env")
	for _, expected = range []string{
		"WorkingDirectory=/srv/project",
		"Environment=NARU_PATH=/var/lib/mininaru",
		"EnvironmentFile=/etc/mininaru/env",
		"ExecStart=/opt/mininaru serve",
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("unit does not contain %q:\n%s", expected, unit)
		}
	}
}
