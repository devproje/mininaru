// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestLegacyClientConfigGetsDefaultContextBudget(t *testing.T) {
	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(util.Path(CLIENT_PATH), []byte(`{"thinking":{"level":"off","show":true}}`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = ClientInit()
	if err != nil {
		t.Fatal(err)
	}
	if Client.Context.MaxChars != 32768 {
		t.Fatalf("context max chars = %d, want default 32768", Client.Context.MaxChars)
	}
	if !Client.Tools.Enabled {
		t.Fatal("tools should default to enabled for legacy client config")
	}
}

func TestLegacyClientConfigKeepsCompactionOn(t *testing.T) {
	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(util.Path(CLIENT_PATH), []byte(`{"context":{"max_chars":4096}}`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = ClientInit()
	if err != nil {
		t.Fatal(err)
	}
	if Client.Context.MaxChars != 4096 {
		t.Fatalf("context max chars = %d, want the configured 4096", Client.Context.MaxChars)
	}
	if !Client.Context.Compact {
		t.Fatal("a config written before compaction existed should still get it on")
	}
}

func TestCompactionCanBeTurnedOffInConfig(t *testing.T) {
	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(util.Path(CLIENT_PATH), []byte(`{"context":{"max_chars":4096,"compact":false}}`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = ClientInit()
	if err != nil {
		t.Fatal(err)
	}
	if Client.Context.Compact {
		t.Fatal("compact false in the config was not honoured")
	}
}
