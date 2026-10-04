// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package browser

import (
	"testing"

	"github.com/devproje/mininaru/util"
)

func setupBrowserConfigFS(t *testing.T) {
	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigDefaultsToZeroValueWhenAbsent(t *testing.T) {
	var config Config

	var err error

	setupBrowserConfigFS(t)

	config, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Channel != "" {
		t.Fatalf("channel = %q, want empty when browser.json doesn't exist", config.Channel)
	}
}

func TestSaveConfigThenLoadConfigRoundtrips(t *testing.T) {
	var config Config

	var err error

	setupBrowserConfigFS(t)

	err = SaveConfig(Config{Channel: ChannelEdge})
	if err != nil {
		t.Fatal(err)
	}

	config, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Channel != ChannelEdge {
		t.Fatalf("channel = %q, want %q", config.Channel, ChannelEdge)
	}
}

func TestSaveConfigRejectsAnUnknownChannel(t *testing.T) {
	var err error

	setupBrowserConfigFS(t)

	err = SaveConfig(Config{Channel: "safari"})
	if err == nil {
		t.Fatal("SaveConfig accepted an unknown channel")
	}
}

func TestValidChannel(t *testing.T) {
	if !ValidChannel("") || !ValidChannel(ChannelChrome) || !ValidChannel(ChannelChromium) || !ValidChannel(ChannelEdge) || !ValidChannel(ChannelBrave) {
		t.Fatal("a known channel (or empty, meaning unset) was rejected")
	}
	if ValidChannel("safari") {
		t.Fatal("an unknown channel was accepted")
	}
}
