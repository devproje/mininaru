// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

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

func TestProviderUpdateNoFieldsIsNoop(t *testing.T) {
	var got *Provider

	var err error

	setupTestDB(t)

	err = ProviderCreate(&Provider{Id: "p1", Name: "one", ApiKey: "key1", BaseUrl: "https://one.example"})
	if err != nil {
		t.Fatal(err)
	}

	err = ProviderUpdate("p1", &Provider{})
	if err != nil {
		t.Fatalf("update with no fields failed: %v", err)
	}

	got, err = ProviderRead("p1")
	if err != nil {
		t.Fatalf("read after no-op update failed: %v", err)
	}
	if got.Name != "one" || got.ApiKey != "key1" || got.BaseUrl != "https://one.example" {
		t.Fatalf("read = %+v, row changed after a no-op update", got)
	}
}

func TestProviderByName(t *testing.T) {
	var got *Provider

	var err error

	setupTestDB(t)

	err = ProviderCreate(&Provider{Id: "p1", Name: "one", ApiKey: "key1", BaseUrl: "https://one.example"})
	if err != nil {
		t.Fatal(err)
	}

	got, err = ProviderByName("one")
	if err != nil {
		t.Fatalf("ProviderByName failed: %v", err)
	}
	if got.Id != "p1" {
		t.Fatalf("id = %q, want p1", got.Id)
	}
}
