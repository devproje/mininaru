// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"testing"
)

func TestWebProviderCreateRequiresFields(t *testing.T) {
	var err error

	setupTestDB(t)

	err = WebProviderCreate(&WebProvider{Name: "one", Kind: "brave"})
	if err == nil {
		t.Fatal("expected an error for a missing id")
	}

	err = WebProviderCreate(&WebProvider{Id: "w1", Kind: "brave"})
	if err == nil {
		t.Fatal("expected an error for a missing name")
	}

	err = WebProviderCreate(&WebProvider{Id: "w1", Name: "one"})
	if err == nil {
		t.Fatal("expected an error for a missing kind")
	}
}

func TestWebProviderCRUD(t *testing.T) {
	var got *WebProvider

	var err error

	setupTestDB(t)

	err = WebProviderCreate(&WebProvider{Id: "w1", Name: "one", Kind: "brave", ApiKey: "key1", BaseUrl: "https://one.example"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err = WebProviderRead("w1")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.Name != "one" || got.Kind != "brave" || got.ApiKey != "key1" || got.BaseUrl != "https://one.example" {
		t.Fatalf("read = %+v, unexpected values", got)
	}

	err = WebProviderUpdate("w1", &WebProvider{Name: "renamed"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err = WebProviderRead("w1")
	if err != nil {
		t.Fatalf("read after update failed: %v", err)
	}
	if got.Name != "renamed" {
		t.Fatalf("name = %q after update, want %q", got.Name, "renamed")
	}

	err = WebProviderDelete("w1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = WebProviderRead("w1")
	if err == nil {
		t.Fatal("expected an error reading a deleted web provider")
	}
}

func TestWebProviderUpdateNoFieldsIsNoop(t *testing.T) {
	var got *WebProvider

	var err error

	setupTestDB(t)

	err = WebProviderCreate(&WebProvider{Id: "w1", Name: "one", Kind: "brave", ApiKey: "key1", BaseUrl: "https://one.example"})
	if err != nil {
		t.Fatal(err)
	}

	err = WebProviderUpdate("w1", &WebProvider{})
	if err != nil {
		t.Fatalf("update with no fields failed: %v", err)
	}

	got, err = WebProviderRead("w1")
	if err != nil {
		t.Fatalf("read after no-op update failed: %v", err)
	}
	if got.Name != "one" || got.Kind != "brave" || got.ApiKey != "key1" || got.BaseUrl != "https://one.example" {
		t.Fatalf("read = %+v, row changed after a no-op update", got)
	}
}

func TestWebProviderByName(t *testing.T) {
	var got *WebProvider

	var err error

	setupTestDB(t)

	err = WebProviderCreate(&WebProvider{Id: "w1", Name: "one", Kind: "brave", ApiKey: "key1"})
	if err != nil {
		t.Fatal(err)
	}

	got, err = WebProviderByName("one")
	if err != nil {
		t.Fatalf("WebProviderByName failed: %v", err)
	}
	if got.Id != "w1" {
		t.Fatalf("id = %q, want w1", got.Id)
	}
}

func TestWebProviderCreateAutoSelectsFirst(t *testing.T) {
	var got *WebProvider

	var err error

	setupTestDB(t)

	err = WebProviderCreate(&WebProvider{Id: "w1", Name: "one", Kind: "brave"})
	if err != nil {
		t.Fatal(err)
	}

	got, err = WebProviderRead("w1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Selected {
		t.Fatal("the first web provider created should be auto-selected")
	}

	err = WebProviderCreate(&WebProvider{Id: "w2", Name: "two", Kind: "tavily"})
	if err != nil {
		t.Fatal(err)
	}

	got, err = WebProviderRead("w2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Selected {
		t.Fatal("a second web provider should not be auto-selected")
	}
}

func TestWebProviderSelect(t *testing.T) {
	var selected *WebProvider

	var err error

	setupTestDB(t)

	err = WebProviderCreate(&WebProvider{Id: "w1", Name: "one", Kind: "brave"})
	if err != nil {
		t.Fatal(err)
	}
	err = WebProviderCreate(&WebProvider{Id: "w2", Name: "two", Kind: "tavily"})
	if err != nil {
		t.Fatal(err)
	}

	selected, err = WebProviderSelected()
	if err != nil {
		t.Fatalf("selected read failed: %v", err)
	}
	if selected.Id != "w1" {
		t.Fatalf("selected = %q, want w1", selected.Id)
	}

	err = WebProviderSelect("w2")
	if err != nil {
		t.Fatalf("select w2 failed: %v", err)
	}

	selected, err = WebProviderSelected()
	if err != nil {
		t.Fatalf("selected read after switch failed: %v", err)
	}
	if selected.Id != "w2" {
		t.Fatalf("selected = %q, want w2", selected.Id)
	}
}

func TestWebProviderSelectedErrorsWithNoneConfigured(t *testing.T) {
	var err error

	setupTestDB(t)

	_, err = WebProviderSelected()
	if err == nil {
		t.Fatal("expected an error when no web provider is selected")
	}
}
