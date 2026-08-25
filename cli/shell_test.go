// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestIsLoopbackURL(t *testing.T) {
	var cases map[string]bool
	var endpoint string
	var want bool

	cases = map[string]bool{
		"ws://127.0.0.1:8223/ws": true,
		"ws://localhost:8223/ws": true,
		"ws://[::1]:8223/ws":     true,
		"ws://example.com/ws":    false,
		"not a url":              false,
	}

	for endpoint, want = range cases {
		if isLoopbackURL(endpoint) != want {
			t.Fatalf("isLoopbackURL(%q) = %v, want %v", endpoint, isLoopbackURL(endpoint), want)
		}
	}
}

func TestResolveApiKeyPrefersExplicitFlag(t *testing.T) {
	var got string

	os.Setenv("MININARU_API_KEY", "from-env")
	defer os.Unsetenv("MININARU_API_KEY")

	got = resolveApiKey("from-flag", "ws://127.0.0.1:8223/ws")
	if got != "from-flag" {
		t.Fatalf("got %q, want the explicit flag to win", got)
	}
}

func TestResolveApiKeyFallsBackToEnv(t *testing.T) {
	var got string

	os.Setenv("MININARU_API_KEY", "from-env")
	defer os.Unsetenv("MININARU_API_KEY")

	got = resolveApiKey("", "ws://example.com:8223/ws")
	if got != "from-env" {
		t.Fatalf("got %q, want the env var", got)
	}
}

func TestResolveApiKeyReadsLocalFileOnlyForLoopback(t *testing.T) {
	var got string

	var err error

	os.Unsetenv("MININARU_API_KEY")

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_, err = util.APIKey()
	if err != nil {
		t.Fatal(err)
	}

	got = resolveApiKey("", "ws://example.com:8223/ws")
	if got != "" {
		t.Fatalf("got %q, want empty for a non-loopback url with no flag/env", got)
	}

	got = resolveApiKey("", "ws://127.0.0.1:8223/ws")
	if got == "" {
		t.Fatal("expected the local key file to be picked up for a loopback url")
	}
}
