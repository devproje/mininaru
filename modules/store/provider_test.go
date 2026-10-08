// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
)

func TestProviderAPIKeyEncryptedAtRest(t *testing.T) {
	var oldRoot string
	var dir string
	var oldDB *sql.DB
	var db *sql.DB
	var id string
	var stored string
	var provider Provider
	var providers []Provider
	var encoded []byte

	var err error

	oldRoot = util.RootDir
	dir = t.TempDir()
	oldDB = util.DB
	defer func() {
		InvalidateModels()
		util.DB = oldDB
		util.RootDir = oldRoot
	}()

	err = util.InitFS(dir)
	if err != nil {
		t.Fatal(err)
	}

	db, err = util.NewDatabase(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	util.DB = db

	id, err = AddProvider(context.Background(), "test", "test-provider-key", "")
	if err != nil {
		t.Fatal(err)
	}

	err = db.QueryRow("SELECT api_key FROM providers WHERE id = ?", id).Scan(&stored)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "enc:v1:") || strings.Contains(stored, "test-provider-key") {
		t.Fatal("provider API key was not encrypted in the database")
	}

	provider, err = GetProvider(id)
	if err != nil {
		t.Fatal(err)
	}
	if provider.ApiKey != "test-provider-key" {
		t.Fatal("provider API key did not decrypt")
	}

	providers, err = GetProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].ApiKey != "test-provider-key" {
		t.Fatal("provider list did not decrypt the API key")
	}

	encoded, err = json.Marshal(provider)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "api_key") || strings.Contains(string(encoded), "test-provider-key") {
		t.Fatal("provider JSON exposed the API key")
	}

	err = SetProvider(context.Background(), id, &Provider{ApiKey: "updated-provider-key"})
	if err != nil {
		t.Fatal(err)
	}

	err = db.QueryRow("SELECT api_key FROM providers WHERE id = ?", id).Scan(&stored)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "enc:v1:") || strings.Contains(stored, "updated-provider-key") {
		t.Fatal("updated provider API key was not encrypted in the database")
	}

	provider, err = GetProvider(id)
	if err != nil {
		t.Fatal(err)
	}
	if provider.ApiKey != "updated-provider-key" {
		t.Fatal("updated provider API key did not decrypt")
	}
}

func TestGetProviderByModel(t *testing.T) {
	var oldDB *sql.DB
	var db *sql.DB
	var provider Provider
	var model string

	var err error

	oldDB = util.DB
	db, err = util.NewDatabase(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	util.DB = db
	defer func() {
		util.DB = oldDB
		db.Close()
	}()

	_, err = db.Exec("INSERT INTO providers (id, name) VALUES (?, ?)", "provider-1", "local")
	if err != nil {
		t.Fatal(err)
	}

	provider, err = GetProviderByModel("local:family:model")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Id != "provider-1" || provider.Name != "local" {
		t.Fatalf("unexpected provider: %+v", provider)
	}

	for _, model = range []string{"", "local", ":model", "local:", " local:model", "local: model"} {
		_, err = GetProviderByModel(model)
		if !errors.Is(err, ErrModelNotFound) {
			t.Errorf("model %q: expected ErrModelNotFound, got %v", model, err)
		}
	}

	_, err = GetProviderByModel("missing:model")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for unknown provider, got %v", err)
	}
}
