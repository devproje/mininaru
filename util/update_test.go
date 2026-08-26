// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"os"
	"testing"
)

func setupTestUpdateVersion(t *testing.T, version string) {
	var previous string

	t.Helper()

	previous = AppVersion
	AppVersion = version

	t.Cleanup(func() { AppVersion = previous })
}

func TestUpdateCacheRoundTrips(t *testing.T) {
	var cache UpdateCache

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	UpdateCacheWrite("v1.0.0-alpha.2")

	cache = UpdateCacheRead()
	if cache.Tag != "v1.0.0-alpha.2" {
		t.Fatalf("cache.Tag = %q, want %q", cache.Tag, "v1.0.0-alpha.2")
	}
	if cache.CheckedAt == 0 {
		t.Fatal("cache.CheckedAt was not set")
	}
}

func TestUpdateCacheReadDefaultsToZeroValueWhenNoFileExists(t *testing.T) {
	var cache UpdateCache

	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cache = UpdateCacheRead()
	if cache.Tag != "" || cache.CheckedAt != 0 {
		t.Fatalf("cache = %+v, want the zero value with no file on disk", cache)
	}
}

func TestUpdateNoticeIsEmptyForADevBuild(t *testing.T) {
	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	setupTestUpdateVersion(t, "dev")
	UpdateCacheWrite("v1.0.0-alpha.2")

	if UpdateNotice() != "" {
		t.Fatalf("UpdateNotice() = %q, want empty for a dev build", UpdateNotice())
	}
}

func TestUpdateNoticeIsEmptyWhenTheEnvOptOutIsSet(t *testing.T) {
	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	setupTestUpdateVersion(t, "v1.0.0-alpha.1")
	UpdateCacheWrite("v1.0.0-alpha.2")

	os.Setenv("MININARU_NO_UPDATE_CHECK", "1")
	t.Cleanup(func() { os.Unsetenv("MININARU_NO_UPDATE_CHECK") })

	if UpdateNotice() != "" {
		t.Fatalf("UpdateNotice() = %q, want empty when opted out", UpdateNotice())
	}
}

func TestUpdateNoticeIsEmptyWithNoCache(t *testing.T) {
	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	setupTestUpdateVersion(t, "v1.0.0-alpha.1")

	if UpdateNotice() != "" {
		t.Fatalf("UpdateNotice() = %q, want empty with no cached check", UpdateNotice())
	}
}

func TestUpdateNoticeIsEmptyWhenAlreadyOnTheCachedTag(t *testing.T) {
	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	setupTestUpdateVersion(t, "v1.0.0-alpha.2")
	UpdateCacheWrite("v1.0.0-alpha.2")

	if UpdateNotice() != "" {
		t.Fatalf("UpdateNotice() = %q, want empty when already on the cached tag", UpdateNotice())
	}
}

func TestUpdateNoticeMentionsTheNewerTag(t *testing.T) {
	var err error

	err = InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	setupTestUpdateVersion(t, "v1.0.0-alpha.1")
	UpdateCacheWrite("v1.0.0-alpha.2")

	if UpdateNotice() == "" {
		t.Fatal("UpdateNotice() was empty, want a notice mentioning the newer tag")
	}
}
