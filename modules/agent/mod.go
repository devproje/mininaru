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

	"git.wh64.net/naru-studio/mininaru/modules/database"
)

type AgentModule struct {
	DB *sql.DB
}

type AgentData struct {
	Id           string             `json:"id"`
	Name         string             `json:"name"`
	Engine       *AgentEngine       `json:"engine"`
	Default      bool               `json:"default"`
	Instructions []AgentInstruction `json:"instructions"`
}

func (m *AgentModule) Name() string {
	return "agent-module"
}

func (m *AgentModule) Load() error {
	if m.DB != nil {
		return nil
	}

	m.DB = database.Database.DB
	return nil
}

func (m *AgentModule) Unload() error {
	if m.DB == nil {
		return nil
	}

	m.DB = nil
	return nil
}

func (m *AgentModule) Create(engineId string, payload *AgentData) error {
	var exists bool = false
	var defaults = false
	var rows *sql.Rows
	var cnt int = 0
	var tx *sql.Tx
	var err error

	exists = m.ExistEngine(engineId)
	if !exists {
		err = fmt.Errorf("engine '%s' is not exists", engineId)
		return err
	}

	tx, err = m.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err = tx.Query("SELECT COUNT(*) FROM agents;")
	if err != nil {
		return err
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&cnt)
		if err != nil {
			return err
		}
	}

	defaults = cnt == 0

	_, err = tx.Exec(
		"INSERT INTO agents (id, `name`, engine, `default`) VALUES (?, ?, ?, ?)",
		payload.Id,
		payload.Name,
		engineId,
		defaults,
	)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) Read(id string) (*AgentData, error) {
	var engineId string
	var data AgentData
	var rows *sql.Rows
	var err error

	rows, err = m.DB.Query("SELECT id, `name`, engine, `default` FROM agents WHERE id = ?;", id)
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		err = fmt.Errorf("agent '%s' is not exists", id)
		return nil, err
	}

	err = rows.Scan(&data.Id, &data.Name, &engineId, &data.Default)
	if err != nil {
		return nil, err
	}

	data.Engine, err = m.ReadEngine(engineId)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (m *AgentModule) GetDefault() (*AgentData, error) {
	var engineId string
	var data AgentData
	var rows *sql.Rows
	var err error

	rows, err = m.DB.Query("SELECT * FROM agents WHERE `default` = 1;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		err = fmt.Errorf("default agent is not exists")
		return nil, err
	}

	err = rows.Scan(&data.Id, &data.Name, &engineId, &data.Default)
	if err != nil {
		return nil, err
	}

	data.Engine, err = m.ReadEngine(engineId)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (m *AgentModule) Exist(id string) bool {
	var rows *sql.Rows
	var cnt int = 0
	var err error

	rows, err = m.DB.Query("SELECT COUNT(*) FROM agents WHERE id = ?;", id)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[Agent] error occurred while checking agent is exists:\n%v\n", err)

		return false
	}
	defer rows.Close()

	if !rows.Next() {
		return false
	}

	err = rows.Scan(&cnt)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[Agent] error occurred while checking agent is exists:\n%v\n", err)

		return false
	}

	return cnt >= 1
}

func (m *AgentModule) SetName(id string, newname string) error {
	var err error

	_, err = m.DB.Exec("UPDATE agents SET name = ? WHERE id = ?;", newname, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) SetEngine(id string, engineId string) error {
	var exists bool = false
	var err error

	exists = m.ExistEngine(engineId)
	if !exists {
		err = fmt.Errorf("engine '%s' is not exists", engineId)
		return err
	}

	_, err = m.DB.Exec("UPDATE agents SET engine = ? WHERE id = ?;", engineId, id)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) SetDefault(id string) error {
	var tx *sql.Tx
	var err error

	tx, err = m.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE agents SET `default` = 0;")
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE agents SET `default` = 1 WHERE id = ?;", id)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) Delete(id string) error {
	var err error

	_, err = m.DB.Exec("DELETE FROM agents WHERE id = ?;", id)
	if err != nil {
		return err
	}

	return nil
}

var Agent *AgentModule = &AgentModule{}
