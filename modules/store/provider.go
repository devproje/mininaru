// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type Provider struct {
	Id      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	ApiKey  string `json:"-"`
	BaseUrl string `json:"base_url,omitempty"`
}

func providerExists(name string) (bool, error) {
	var id string

	var err error

	err = util.DB.QueryRow("SELECT id FROM providers WHERE name = ?;", name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func AddProvider(ctx context.Context, name, apiKey, baseUrl string) (string, error) {
	var id string
	var encryptedKey string
	var tx *sql.Tx

	var err error

	id = uuid.NewString()
	encryptedKey, err = util.Encrypt(apiKey)
	if err != nil {
		return "", err
	}

	tx, err = util.DB.Begin()
	if err != nil {
		return "", err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO providers (id, name, api_key, base_url) VALUES (?, ?, ?, ?);",
		id, name, encryptedKey, baseUrl)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}

	err = tx.Commit()
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}

	InvalidateModels()

	return id, nil
}

func GetProviders() ([]Provider, error) {
	var prov []Provider
	var row *sql.Rows
	var cur Provider

	var err error

	row, err = util.DB.Query("SELECT id, name, api_key, base_url FROM providers ORDER BY name ASC;")
	if err != nil {
		return []Provider{}, err
	}
	defer row.Close()

	for row.Next() {
		cur = Provider{}
		err = row.Scan(&cur.Id, &cur.Name, &cur.ApiKey, &cur.BaseUrl)
		if err != nil {
			return []Provider{}, err
		}
		cur.ApiKey, err = util.Decrypt(cur.ApiKey)
		if err != nil {
			return []Provider{}, err
		}

		prov = append(prov, cur)
	}
	err = row.Err()
	if err != nil {
		return []Provider{}, err
	}

	return prov, nil
}

func GetProvider(id string) (Provider, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var prov Provider

	var err error

	stmt, err = util.DB.Prepare("SELECT id, name, api_key, base_url FROM providers WHERE id = ?;")
	if err != nil {
		return Provider{}, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(id)
	err = row.Scan(&prov.Id, &prov.Name, &prov.ApiKey, &prov.BaseUrl)
	if err != nil {
		return Provider{}, err
	}
	prov.ApiKey, err = util.Decrypt(prov.ApiKey)
	if err != nil {
		return Provider{}, err
	}

	err = row.Err()
	if err != nil {
		return Provider{}, err
	}

	return prov, err
}

func GetProviderByName(name string) (Provider, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var prov Provider

	var err error

	stmt, err = util.DB.Prepare("SELECT id, name, api_key, base_url FROM providers WHERE name = ?;")
	if err != nil {
		return Provider{}, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(name)
	err = row.Scan(&prov.Id, &prov.Name, &prov.ApiKey, &prov.BaseUrl)
	if err != nil {
		return Provider{}, err
	}
	prov.ApiKey, err = util.Decrypt(prov.ApiKey)
	if err != nil {
		return Provider{}, err
	}

	err = row.Err()
	if err != nil {
		return Provider{}, err
	}

	return prov, err
}

func GetProviderByModel(model string) (Provider, error) {
	var name string
	var modelName string
	var valid bool
	var prov Provider

	var err error

	name, modelName, valid = strings.Cut(model, ":")
	if !valid || name == "" || modelName == "" ||
		strings.TrimSpace(name) != name || strings.TrimSpace(modelName) != modelName {
		return Provider{}, ErrModelNotFound
	}

	prov, err = GetProviderByName(name)
	if err != nil {
		return Provider{}, err
	}

	return prov, nil
}

func SetProvider(ctx context.Context, id string, prov *Provider) error {
	var tx *sql.Tx
	var query string
	var params []string
	var values []any
	var encryptedKey string

	var err error

	if prov.ApiKey != "" {
		encryptedKey, err = util.Encrypt(prov.ApiKey)
		if err != nil {
			return err
		}
	}

	tx, err = util.DB.Begin()
	if err != nil {
		return err
	}

	if prov.ApiKey != "" {
		params = append(params, "api_key = ?")
		values = append(values, encryptedKey)
	}

	if prov.BaseUrl != "" {
		params = append(params, "base_url = ?")
		values = append(values, prov.BaseUrl)
	}

	if len(params) <= 0 {
		_ = tx.Rollback()
		return ErrNoFieldsToUpdate
	}

	values = append(values, id)

	query = fmt.Sprintf("UPDATE providers SET %s WHERE id = ?;", strings.Join(params, ", "))
	_, err = tx.ExecContext(ctx, query, values...)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	InvalidateModels()
	return nil
}

func SetEmptyApiKey(ctx context.Context, id string) error {
	var err error
	err = emptyHelper(ctx, id, "providers", "api_key")
	if err != nil {
		return err
	}

	InvalidateModels()

	return nil
}

func SetEmptyBaseUrl(ctx context.Context, id string) error {
	var err error
	err = emptyHelper(ctx, id, "providers", "base_url")
	if err != nil {
		return err
	}

	InvalidateModels()

	return nil
}

func RemoveProvider(ctx context.Context, id string) error {
	var tx *sql.Tx

	var err error

	tx, err = util.DB.Begin()
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM providers WHERE id = ?;", id)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	InvalidateModels()

	return nil
}
