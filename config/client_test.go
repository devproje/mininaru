// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestLegacyClientConfigGetsCurrentDefaults(t *testing.T) {
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

func TestClientModeUsesThePairedServer(t *testing.T) {
	Client = defaultClient
	Client.Mode = ModeClient
	Client.Server.Address = "naru.example.com:9090"

	if !RemoteClient() {
		t.Fatal("client mode did not use the paired server")
	}
}

func TestServerModeStaysLocalWithAStoredAddress(t *testing.T) {
	Client = defaultClient
	Client.Mode = ModeServer
	Client.Server.Address = "naru.example.com:9090"

	if RemoteClient() {
		t.Fatal("server mode used a stored client address")
	}
}

func TestLegacyPairedConfigStillUsesTheServer(t *testing.T) {
	Client = defaultClient
	Client.Server.Address = "naru.example.com:9090"

	if !RemoteClient() {
		t.Fatal("legacy paired config stopped using the server")
	}
}
