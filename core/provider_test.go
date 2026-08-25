// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"
)

func TestProviderCreateRequiresIdAndName(t *testing.T) {
	var err error

	setupTestDB(t)

	err = ProviderCreate(&Provider{Name: "one"})
	if err == nil {
		t.Fatal("expected an error for a missing id")
	}

	err = ProviderCreate(&Provider{Id: "p1"})
	if err == nil {
		t.Fatal("expected an error for a missing name")
	}
}

func TestProviderCRUD(t *testing.T) {
	var got *Provider

	var err error

	setupTestDB(t)

	err = ProviderCreate(&Provider{Id: "p1", Name: "one", ApiKey: "key1", BaseUrl: "https://one.example"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err = ProviderRead("p1")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.Name != "one" || got.ApiKey != "key1" || got.BaseUrl != "https://one.example" {
		t.Fatalf("read = %+v, unexpected values", got)
	}
	if got.Active {
		t.Fatal("a newly created provider should not be active")
	}

	err = ProviderUpdate("p1", &Provider{Name: "renamed"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err = ProviderRead("p1")
	if err != nil {
		t.Fatalf("read after update failed: %v", err)
	}
	if got.Name != "renamed" {
		t.Fatalf("name = %q after update, want %q", got.Name, "renamed")
	}

	err = ProviderDelete("p1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = ProviderRead("p1")
	if err == nil {
		t.Fatal("expected an error reading a deleted provider")
	}
}

func TestProviderActivateSwitchesActiveProvider(t *testing.T) {
	var active *Provider

	var err error

	setupTestDB(t)

	err = ProviderCreate(&Provider{Id: "p1", Name: "one"})
	if err != nil {
		t.Fatal(err)
	}
	err = ProviderCreate(&Provider{Id: "p2", Name: "two"})
	if err != nil {
		t.Fatal(err)
	}

	err = ProviderActivate("p1")
	if err != nil {
		t.Fatalf("activate p1 failed: %v", err)
	}

	active, err = ProviderActive()
	if err != nil {
		t.Fatalf("active read failed: %v", err)
	}
	if active.Id != "p1" {
		t.Fatalf("active = %q, want p1", active.Id)
	}

	err = ProviderActivate("p2")
	if err != nil {
		t.Fatalf("activate p2 failed: %v", err)
	}

	active, err = ProviderActive()
	if err != nil {
		t.Fatalf("active read after switch failed: %v", err)
	}
	if active.Id != "p2" {
		t.Fatalf("active = %q, want p2", active.Id)
	}
}
