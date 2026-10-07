// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"context"
	"database/sql"
	"encoding/json"
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
		invalidateModels()
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
