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

type AgentRole string

const (
	USER      AgentRole = "user"
	ASSISTANT AgentRole = "assistant"
)

type AgentSession struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionContext struct {
	Id        string    `json:"id"`
	SessionId string    `json:"session_id"`
	AgentId   string    `json:"agent_id"`
	Role      AgentRole `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *AgentModule) CreateSession(agentId string, payload *AgentSession) error {
	var err error

	if !m.Exist(agentId) {
		err = fmt.Errorf("agent '%s' is not exists.", agentId)
		goto handle_err
	}

	_, err = m.DB.Exec("INSERT INTO agent_session (id, agent_id, name) VALUES (?, ?, ?);", payload.Id, agentId, "Untitled")
	if err != nil {
		goto handle_err
	}

	return nil

handle_err:
	return err
}

func (m *AgentModule) ReadSession(id, agentId string) (*AgentSession, error) {
	var err error
	var rows *sql.Rows
	var session AgentSession

	rows, err = m.DB.Query("SELECT id, name, created_at, updated_at FROM agent_session WHERE id = ? AND agent_id = ?;", id, agentId)
	if err != nil {
		goto handle_err
	}
	defer rows.Close()

	if !rows.Next() {
		err = fmt.Errorf("session '%s' is not exists.", id)
		goto handle_err
	}

	err = rows.Scan(&session.Id, &session.Name, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		goto handle_err
	}

	return &session, nil

handle_err:
	return nil, err
}

func (m *AgentModule) RenameSession(id, agentId, newname string) error {
	var err error

	_, err = m.DB.Exec("UPDATE agent_session SET name = ? WHERE id = ? AND agent_id = ?;", id, agentId)
	if err != nil {
		return err
	}

	return nil
}

func (m *AgentModule) DeleteSession(id, agentId string) error {
	var err error

	_, err = m.DB.Exec("DELETE FROM agent_session WHERE id = ? AND agent_id = ?;", id, agentId)
	if err != nil {
		return err
	}

	return nil
}
