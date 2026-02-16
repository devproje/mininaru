// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * MiniNaru
 * Copyright (C) 2022-2026 Project_IO
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package agent

import (
	"database/sql"
	"fmt"
	"os"
)

type AgentEngine struct {
	Id          string `json:"id"`
	ApiEndpoint string `json:"api_endpoint"`
	ApiKey      string `json:"api_key"`
	Model       string `json:"model"`
}

func (m *AgentModule) CreateEngine(payload *AgentEngine) error {
	var err error

	_, err = m.DB.Exec(
		"INSERT INTO agent_engine (id, api_endpoint, api_key, model) VALUES (?, ?, ?, ?)",
		payload.Id,
		payload.ApiEndpoint,
		payload.ApiKey,
		payload.Model,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) ReadEngine(id string) (*AgentEngine, error) {
	var engine AgentEngine
	var rows *sql.Rows
	var err error

	rows, err = m.DB.Query("SELECT `name`, api_endpoint, api_key, model FROM agent_engine WHERE name = ?", id)
	if err != nil {
		goto err_cleanup
	}

	if !rows.Next() {
		err = fmt.Errorf("engine '%s' is not exists", id)
		goto err_row_cleanup
	}

	err = rows.Scan(&engine.Id, &engine.ApiEndpoint, &engine.ApiKey, &engine.Model)
	if err != nil {
		goto err_row_cleanup
	}

	rows.Close()

	return &engine, nil

err_row_cleanup:
	rows.Close()
err_cleanup:
	return nil, err
}

func (m *AgentModule) ExistEngine(id string) bool {
	var rows *sql.Rows
	var cnt int = 0
	var err error

	rows, err = m.DB.Query("SELECT COUNT(*) FROM agent_engine WHERE id = ?;", id)
	if err != nil {
		goto err_cleanup
	}

	if !rows.Next() {
		goto err_row_cleanup
	}

	err = rows.Scan(&cnt)
	if err != nil {
		goto err_row_cleanup
	}

	rows.Close()

	return cnt >= 1

err_row_cleanup:
	rows.Close()
err_cleanup:
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[agent]: error occurred while checking model is exists:\n%v\n", err)
	}

	return false
}

func (m *AgentModule) RenameEngine(id, newid string) error {
	var err error

	_, err = m.DB.Exec("UPDATE agent_engine SET name = ? WHERE name = ?;", newid, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) UpdateEndpointEngine(id, endpoint string) error {
	var err error

	_, err = m.DB.Exec("UPDATE agent_engine SET api_endpoint = ? WHERE name = ?", endpoint, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) UpdateKeyEngine(id, key string) error {
	var err error

	_, err = m.DB.Exec("UPDATE agent_engine SET api_key = ? WHERE name = ?;", key, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) UpdateModelEngine(id, model string) error {
	var err error

	_, err = m.DB.Exec("UPDATE agent_engine SET model = ? WHERE name = ?;", model, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) DeleteEngine(id string) error {
	var err error

	_, err = m.DB.Exec("DELETE FROM agent_engine WHERE name = ?;", id)
	if err != nil {
		return err
	}

	return nil
}
