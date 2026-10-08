// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestUpsertProfileUsesLocalProviderAndPreservesFields(t *testing.T) {
	oldRoot := util.RootDir
	oldDB := util.DB
	defer func() {
		util.RootDir = oldRoot
		util.DB = oldDB
	}()

	dir := t.TempDir()
	if err := util.InitFS(dir); err != nil {
		t.Fatal(err)
	}
	db, err := util.NewDatabase(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	util.DB = db

	if _, err := db.Exec("INSERT INTO providers (id, name, base_url) VALUES (?, ?, ?)", "provider-1", "local", "http://127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}

	name, model, soul := "Naru", "local:model-without-listing", "original soul"
	if err := UpsertProfile(&Profile{Name: &name, Model: &model, Soul: &soul}); err != nil {
		t.Fatalf("create profile using a registered provider: %v", err)
	}

	updatedName, emptySoul := "Hono", ""
	if err := UpsertProfile(&Profile{Name: &updatedName, Soul: &emptySoul}); err != nil {
		t.Fatalf("update profile: %v", err)
	}
	got, err := GetProfile()
	if err != nil {
		t.Fatal(err)
	}
	if got.Name == nil || *got.Name != updatedName || got.Model == nil || *got.Model != model || got.Soul == nil || *got.Soul != "" {
		t.Fatalf("partial update did not preserve model or clear soul: %+v", got)
	}

	for _, invalid := range []string{"missing:model", "local:", "model-only"} {
		if err := UpsertProfile(&Profile{Model: &invalid}); !errors.Is(err, ErrModelNotFound) {
			t.Errorf("model %q: expected ErrModelNotFound, got %v", invalid, err)
		}
	}
	got, err = GetProfile()
	if err != nil {
		t.Fatal(err)
	}
	if got.Model == nil || *got.Model != model {
		t.Fatalf("invalid model changed saved profile: %+v", got)
	}

	info, err := os.Stat(filepath.Join(dir, profilePath))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("profile permissions: got %o, want 600", info.Mode().Perm())
	}
}
