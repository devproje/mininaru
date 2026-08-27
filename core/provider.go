// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
)

type Provider struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	ApiKey  string `json:"api_key"`
	BaseUrl string `json:"base_url"`
	Active  bool   `json:"active"`
}

func ProviderCreate(prov *Provider) error {
	var opts []string
	var values []any
	var i int
	var wild []string

	var query string
	var stmt *sql.Stmt

	var err error

	opts = []string{"id", "name"}
	values = []any{prov.Id, prov.Name}

	if prov.ApiKey != "" {
		opts = append(opts, "api_key")
		values = append(values, prov.ApiKey)
	}

	if prov.BaseUrl != "" {
		opts = append(opts, "base_url")
		values = append(values, prov.BaseUrl)
	}

	if prov.Id == "" || prov.Name == "" {
		err = fmt.Errorf("provider id or name is required")
		return err
	}

	for i = 0; i < len(opts); i++ {
		wild = append(wild, "?")
	}

	query = fmt.Sprintf("INSERT INTO providers (%s) VALUES (%s);", strings.Join(opts, ", "), strings.Join(wild, ", "))

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

func ProviderRead(id string) (*Provider, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj Provider

	var err error

	stmt, err = util.DB.Prepare("SELECT id, name, api_key, base_url, active FROM providers WHERE id = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(id)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.Name, &obj.ApiKey, &obj.BaseUrl, &obj.Active)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func ProviderList() ([]*Provider, error) {
	var rows *sql.Rows
	var list []*Provider
	var obj Provider

	var err error

	rows, err = util.DB.Query("SELECT id, name, api_key, base_url, active FROM providers ORDER BY name ASC;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&obj.Id, &obj.Name, &obj.ApiKey, &obj.BaseUrl, &obj.Active)
		if err != nil {
			return nil, err
		}

		list = append(list, &Provider{
			Id:      obj.Id,
			Name:    obj.Name,
			ApiKey:  obj.ApiKey,
			BaseUrl: obj.BaseUrl,
			Active:  obj.Active,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func ProviderActive() (*Provider, error) {
	var row *sql.Row
	var obj Provider

	var err error

	row = util.DB.QueryRow("SELECT id, name, api_key, base_url, active FROM providers WHERE active = 1;")
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.Name, &obj.ApiKey, &obj.BaseUrl, &obj.Active)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func ProviderUpdate(id string, prov *Provider) error {
	var opts []string
	var values []any
	var query string

	var stmt *sql.Stmt
	var err error

	if prov.Name != "" {
		opts = append(opts, "name = ?")
		values = append(values, prov.Name)
	}

	if prov.ApiKey != "" {
		opts = append(opts, "api_key = ?")
		values = append(values, prov.ApiKey)
	}

	if prov.BaseUrl != "" {
		opts = append(opts, "base_url = ?")
		values = append(values, prov.BaseUrl)
	}

	values = append(values, id)
	query = fmt.Sprintf("UPDATE providers SET %s WHERE id = ?;", strings.Join(opts, ", "))

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

func ProviderDelete(id string) error {
	var stmt *sql.Stmt
	var err error

	stmt, err = util.DB.Prepare("DELETE FROM providers WHERE id = ?;")
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

func ProviderActivate(id string) error {
	var tx *sql.Tx
	var rollbackErr error

	var err error

	tx, err = util.DB.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE providers SET active = 0 WHERE active = 1;")
	if err != nil {
		rollbackErr = tx.Rollback()
		if rollbackErr != nil {
			return rollbackErr
		}

		return err
	}

	_, err = tx.Exec("UPDATE providers SET active = 1 WHERE id = ?;", id)
	if err != nil {
		rollbackErr = tx.Rollback()
		if rollbackErr != nil {
			return rollbackErr
		}

		return err
	}

	return tx.Commit()
}
