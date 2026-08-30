// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/devproje/mininaru/modules/client"
	"github.com/devproje/mininaru/util"
)

func TestGatewayStoreRoundTrips(t *testing.T) {
	var loaded gatewayStore

	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	loaded, err = loadGateways()
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 0 {
		t.Fatalf("missing file should load empty, got %v", loaded)
	}

	err = saveGateways(gatewayStore{"prod": {Url: "ws://box/ws", ApiKey: "sk-1"}})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err = loadGateways()
	if err != nil {
		t.Fatal(err)
	}

	if loaded["prod"].Url != "ws://box/ws" || loaded["prod"].ApiKey != "sk-1" {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

func TestResolveGateway(t *testing.T) {
	var entry *client.Gateway

	var err error

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	err = saveGateways(gatewayStore{"prod": {Url: "ws://box/ws", ApiKey: "sk-1"}})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { gatewayRef = "" }()

	gatewayRef = ""
	entry, err = resolveGateway()
	if err != nil || entry != nil {
		t.Fatalf("empty ref should resolve to nil, got %v %v", entry, err)
	}

	gatewayRef = "prod"
	entry, err = resolveGateway()
	if err != nil {
		t.Fatal(err)
	}

	if entry.Url != "ws://box/ws" || entry.ApiKey != "sk-1" || entry.Name != "prod" {
		t.Fatalf("resolved wrong entry: %+v", entry)
	}

	gatewayRef = "missing"
	_, err = resolveGateway()
	if err == nil {
		t.Fatal("unknown gateway should error")
	}
}

func TestGatewayNameValidation(t *testing.T) {
	var name string

	for _, name = range []string{"prod", "dev-1", "a_b", "X"} {
		if !gatewayNamePattern.MatchString(name) {
			t.Errorf("%q should be valid", name)
		}
	}

	for _, name = range []string{"", "bad name", "with/slash", "way-too-long-a-name-for-a-gateway-entry"} {
		if gatewayNamePattern.MatchString(name) {
			t.Errorf("%q should be invalid", name)
		}
	}
}
