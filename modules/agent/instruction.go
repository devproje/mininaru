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
	"time"
)

type AgentInstruction struct {
	Filename  string    `json:"filename"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *AgentModule) CreateInstruction(agentId string, payload *AgentInstruction) error {
	var err error

	if !m.Exist(agentId) {
		err = fmt.Errorf("agent '%s' is not exists", agentId)
		goto handle_err
	}

	_, err = m.DB.Exec(
		"INSERT INTO agent_instructions (agent_id, `name`, content) VALUES (?, ?, ?);",
		agentId,
		payload.Filename,
		payload.Content,
	)
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
	return err
}

func (m *AgentModule) ReadInstructions(agentId string) ([]AgentInstruction, error) {
	var instructions = make([]AgentInstruction, 0)
	var instruction AgentInstruction
	var rows *sql.Rows
	var err error

	if !m.Exist(agentId) {
		err = fmt.Errorf("agent '%s' is not exists", agentId)
		goto handle_err
	}

	rows, err = m.DB.Query("SELECT `name`, content, created_at, updated_at FROM agent_instructions WHERE agent_id = ?", agentId)
	if err != nil {
		goto handle_err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&instruction.Filename, &instruction.Content, &instruction.CreatedAt, &instruction.UpdatedAt)
		if err != nil {
			goto handle_err
		}

		instructions = append(instructions, instruction)
	}

	return instructions, nil

handle_err:
	return nil, err
}

func (m *AgentModule) ReadInstruction(agentId, filename string) (*AgentInstruction, error) {
	var instruction AgentInstruction
	var rows *sql.Rows
	var err error

	if !m.Exist(agentId) {
		err = fmt.Errorf("agent '%s' is not exists", agentId)
		goto handle_err
	}

	rows, err = m.DB.Query("SELECT `name`, content, created_at, updated_at FROM agent_instructions WHERE agent_id = ? AND `name` = ?", agentId, filename)
	if err != nil {
		goto handle_err
	}
	defer rows.Close()

	if !rows.Next() {
		err = fmt.Errorf("%s agent instruction name '%s' is not exists", agentId, filename)
		goto handle_err
	}

	err = rows.Scan(&instruction.Filename, &instruction.Content, &instruction.CreatedAt, &instruction.UpdatedAt)
	if err != nil {
		goto handle_err
	}

	return &instruction, nil

handle_err:
	return nil, err
}

func (m *AgentModule) RenameInstruction(agentId, filename, newname string) error {
	var err error

	if !m.Exist(agentId) {
		err = fmt.Errorf("agent '%s' is not exists", agentId)
		goto handle_err
	}

	_, err = m.DB.Exec("UPDATE agent_instructions SET `name` = ? WHERE agent_id = ? AND `name` = ?;", newname, agentId, filename)
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
	return err
}

func (m *AgentModule) UpdateInstruction(agentId, filename, content string) error {
	var err error

	if !m.Exist(agentId) {
		err = fmt.Errorf("agent '%s' is not exists", agentId)
		goto handle_err
	}

	_, err = m.DB.Exec("UPDATE agent_instructions SET content = ? WHERE agent_id = ? AND `name` = ?;", content, agentId, filename)
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
	return err
}

func (m *AgentModule) DeleteInstruction(agentId string, filename string) error {
	var err error

	if !m.Exist(agentId) {
		err = fmt.Errorf("agent '%s' is not exists", agentId)
		goto handle_err
	}

	_, err = m.DB.Exec("DELETE FROM agent_instructions WHERE agent_id = ? AND `name` = ?;", agentId, filename)
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
	return err
}
