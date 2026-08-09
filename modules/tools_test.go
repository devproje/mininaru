// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"strings"
	"testing"
)

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
}
