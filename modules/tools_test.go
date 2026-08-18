// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDefaultToolsAtRootsFileAccessAtTheGivenDirectory(t *testing.T) {
	var root string
	var defs []Def
	var def Def
	var result string

	var err error

	root = t.TempDir()
	err = os.WriteFile(root+"/inside.txt", []byte("home-root"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	defs, err = DefaultToolsAt(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, def = range defs {
		if def.Name != "file_read" {
			continue
		}
		result, err = def.Execute(context.Background(), `{"path":"inside.txt"}`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, "home-root") {
			t.Fatalf("file_read result = %q", result)
		}
		return
	}

	t.Fatal("file_read tool not found")
}

func TestCurrentTime(t *testing.T) {
	var result string

	var err error

	result, err = CurrentTime().Execute(context.Background(), `{"timezone":"Asia/Seoul"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"timezone":"Asia/Seoul"`) || !strings.Contains(result, `"datetime":`) {
		t.Fatalf("current_time result = %q", result)
	}
}

func TestCurrentTimeRejectsInvalidTimezone(t *testing.T) {
	var err error

	_, err = CurrentTime().Execute(context.Background(), `{"timezone":"not/a-zone"}`)
	if err == nil {
		t.Fatal("invalid timezone unexpectedly succeeded")
	}
}

func TestDefaultToolPermissions(t *testing.T) {
	var defs []Def
	var permissions map[string]Permission
	var def Def

	defs = DefaultTools()
	permissions = map[string]Permission{}
	for _, def = range defs {
		permissions[def.Name] = def.Permission
	}

	if permissions["current_time"] != PermissionSafe || permissions["web_search"] != PermissionSafe {
		t.Fatalf("safe tool permissions = %#v", permissions)
	}
	if permissions["file_read"] != PermissionDangerous || permissions["file_write"] != PermissionDangerous || permissions["bash_exec"] != PermissionDangerous {
		t.Fatalf("filesystem tool permissions = %#v", permissions)
	}
	if permissions["file_edit"] != PermissionDangerous || permissions["grep"] != PermissionDangerous || permissions["glob"] != PermissionDangerous {
		t.Fatalf("coding tool permissions = %#v", permissions)
	}
}

func TestSafeToolsExcludeCodingTools(t *testing.T) {
	var def Def
	var offered map[string]bool

	offered = map[string]bool{}
	for _, def = range SafeTools() {
		offered[def.Name] = true
	}

	if offered["grep"] || offered["glob"] || offered["file_edit"] {
		t.Fatalf("safe tools offered a coding tool: %#v", offered)
	}
}
