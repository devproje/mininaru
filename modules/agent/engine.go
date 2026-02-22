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
	"strings"
)

type AgentEngine struct {
	Id          string `json:"id"`
	ApiEndpoint string `json:"api_endpoint"`
	ApiKey      string `json:"api_key"`
	Model       string `json:"model"`
}

type EngineUpdatePayload struct {
	ApiEndpoint *string `json:"api_endpoint"`
	ApiKey      *string `json:"api_key"`
	Model       *string `json:"model"`
}

func (m *AgentModule) CreateEngine(payload *AgentEngine) error {
	var err error

	_, err = m.DB.Exec(
		"INSERT INTO agent_engine (id, api_endpoint, api_key, model) VALUES (?, ?, ?, ?);",
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

	rows, err = m.DB.Query("SELECT id, api_endpoint, api_key, model FROM agent_engine WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		err = fmt.Errorf("engine '%s' is not exists", id)
		return nil, err
	}

	err = rows.Scan(&engine.Id, &engine.ApiEndpoint, &engine.ApiKey, &engine.Model)
	if err != nil {
		return nil, err
	}

	return &engine, nil
}

func (m *AgentModule) ExistEngine(id string) bool {
	var rows *sql.Rows
	var cnt int = 0
	var err error

	rows, err = m.DB.Query("SELECT COUNT(id) FROM agent_engine WHERE id = ?;", id)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[agent]: error occurred while checking model is exists:\n%v\n", err)

		return false
	}
	defer rows.Close()

	if !rows.Next() {
		return false
	}

	err = rows.Scan(&cnt)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[agent]: error occurred while checking model is exists:\n%v\n", err)

		return false
	}

	return cnt >= 1
}

func (m *AgentModule) UpdateEngine(id string, payload *EngineUpdatePayload) error {
	var err error
	var query string

	var sets []string = make([]string, 0)
	var args []any = make([]any, 0)

	if payload.ApiEndpoint != nil {
		sets = append(sets, "api_endpoint = ?")
		args = append(args, *payload.ApiEndpoint)
	}

	if payload.ApiKey != nil {
		sets = append(sets, "api_key = ?")
		args = append(args, *payload.ApiKey)
	}

	if payload.Model != nil {
		sets = append(sets, "model = ?")
		args = append(args, *payload.Model)
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query = fmt.Sprintf("UPDATE agent_engine SET %s WHERE id = ?;", strings.Join(sets, ", "))

	_, err = m.DB.Exec(query, args...)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) DeleteEngine(id string) error {
	var err error

	_, err = m.DB.Exec("DELETE FROM agent_engine WHERE id = ?;", id)
	if err != nil {
		return err
	}

	return nil
}
