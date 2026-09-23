// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
)

type WebProvider struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	ApiKey   string `json:"api_key"`
	BaseUrl  string `json:"base_url"`
	Selected bool   `json:"selected"`
}

func WebProviderCreate(prov *WebProvider) error {
	var opts []string
	var values []any
	var count int
	var apiKey string
	var i int
	var wild []string

	var query string
	var stmt *sql.Stmt

	var err error

	if prov.Id == "" || prov.Name == "" || prov.Kind == "" {
		err = fmt.Errorf("web provider id, name, or kind is required")
		return err
	}

	opts = []string{"id", "name", "kind"}
	values = []any{prov.Id, prov.Name, prov.Kind}

	err = util.DB.QueryRow("SELECT COUNT(*) FROM web_providers;").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		opts = append(opts, "selected")
		values = append(values, true)
	}

	if prov.ApiKey != "" {
		apiKey, err = util.Encrypt(prov.ApiKey)
		if err != nil {
			return err
		}

		opts = append(opts, "api_key")
		values = append(values, apiKey)
	}

	if prov.BaseUrl != "" {
		opts = append(opts, "base_url")
		values = append(values, prov.BaseUrl)
	}

	for i = 0; i < len(opts); i++ {
		wild = append(wild, "?")
	}

	query = fmt.Sprintf("INSERT INTO web_providers (%s) VALUES (%s);", strings.Join(opts, ", "), strings.Join(wild, ", "))

	stmt, err = util.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(values...)
	if err != nil {
		return err
	}

	return nil
}

func WebProviderRead(id string) (*WebProvider, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj WebProvider

	var err error

	stmt, err = util.DB.Prepare("SELECT id, name, kind, api_key, base_url, selected FROM web_providers WHERE id = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(id)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.Name, &obj.Kind, &obj.ApiKey, &obj.BaseUrl, &obj.Selected)
	if err != nil {
		return nil, err
	}

	obj.ApiKey, err = util.Decrypt(obj.ApiKey)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func WebProviderByName(name string) (*WebProvider, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj WebProvider

	var err error

	stmt, err = util.DB.Prepare("SELECT id, name, kind, api_key, base_url, selected FROM web_providers WHERE name = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(name)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.Name, &obj.Kind, &obj.ApiKey, &obj.BaseUrl, &obj.Selected)
	if err != nil {
		return nil, err
	}

	obj.ApiKey, err = util.Decrypt(obj.ApiKey)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func WebProviderList() ([]*WebProvider, error) {
	var rows *sql.Rows
	var list []*WebProvider
	var obj WebProvider

	var err error

	rows, err = util.DB.Query("SELECT id, name, kind, api_key, base_url, selected FROM web_providers ORDER BY name ASC;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&obj.Id, &obj.Name, &obj.Kind, &obj.ApiKey, &obj.BaseUrl, &obj.Selected)
		if err != nil {
			return nil, err
		}

		obj.ApiKey, err = util.Decrypt(obj.ApiKey)
		if err != nil {
			return nil, err
		}

		list = append(list, &WebProvider{
			Id:       obj.Id,
			Name:     obj.Name,
			Kind:     obj.Kind,
			ApiKey:   obj.ApiKey,
			BaseUrl:  obj.BaseUrl,
			Selected: obj.Selected,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func WebProviderUpdate(id string, prov *WebProvider) error {
	var opts []string
	var values []any
	var apiKey string
	var query string

	var stmt *sql.Stmt
	var err error

	if prov.Name != "" {
		opts = append(opts, "name = ?")
		values = append(values, prov.Name)
	}

	if prov.Kind != "" {
		opts = append(opts, "kind = ?")
		values = append(values, prov.Kind)
	}

	if prov.ApiKey != "" {
		apiKey, err = util.Encrypt(prov.ApiKey)
		if err != nil {
			return err
		}

		opts = append(opts, "api_key = ?")
		values = append(values, apiKey)
	}

	if prov.BaseUrl != "" {
		opts = append(opts, "base_url = ?")
		values = append(values, prov.BaseUrl)
	}

	if len(opts) == 0 {
		return nil
	}

	values = append(values, id)
	query = fmt.Sprintf("UPDATE web_providers SET %s WHERE id = ?;", strings.Join(opts, ", "))

	stmt, err = util.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(values...)
	if err != nil {
		return err
	}

	return nil
}

func WebProviderDelete(id string) error {
	var stmt *sql.Stmt
	var err error

	stmt, err = util.DB.Prepare("DELETE FROM web_providers WHERE id = ?;")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(id)
	if err != nil {
		return err
	}

	return nil
}

func WebProviderSelect(id string) error {
	var tx *sql.Tx
	var rollbackErr error

	var err error

	tx, err = util.DB.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE web_providers SET selected = 0 WHERE selected = 1;")
	if err != nil {
		rollbackErr = tx.Rollback()
		if rollbackErr != nil {
			return rollbackErr
		}

		return err
	}

	_, err = tx.Exec("UPDATE web_providers SET selected = 1 WHERE id = ?;", id)
	if err != nil {
		rollbackErr = tx.Rollback()
		if rollbackErr != nil {
			return rollbackErr
		}

		return err
	}

	return tx.Commit()
}

func WebProviderSelected() (*WebProvider, error) {
	var row *sql.Row
	var obj WebProvider

	var err error

	row = util.DB.QueryRow("SELECT id, name, kind, api_key, base_url, selected FROM web_providers WHERE selected = 1;")
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.Name, &obj.Kind, &obj.ApiKey, &obj.BaseUrl, &obj.Selected)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no web provider is selected — select one with `mininaru webprovider primary <id-or-name>`")
		}

		return nil, err
	}

	obj.ApiKey, err = util.Decrypt(obj.ApiKey)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}
