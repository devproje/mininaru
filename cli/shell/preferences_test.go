// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"testing"
)

func TestLoadPreferencesDefaultsToEmptyWhenNoFileExists(t *testing.T) {
	var prefs *preferences

	var err error

	setupTestNaruPath(t)

	prefs, err = loadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if prefs.Agent != "" {
		t.Fatalf("prefs.Agent = %q, want empty with no file on disk", prefs.Agent)
	}
}

func TestSavePreferencesRoundTrips(t *testing.T) {
	var prefs *preferences

	var err error

	setupTestNaruPath(t)

	err = savePreferences(&preferences{Agent: "worker"})
	if err != nil {
		t.Fatal(err)
	}

	prefs, err = loadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if prefs.Agent != "worker" {
		t.Fatalf("prefs.Agent = %q, want %q", prefs.Agent, "worker")
	}
}
