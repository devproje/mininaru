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

package chat

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ChatChannel struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	AgentId   string    `json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateChatChannel struct {
	Name    *string `json:"name"`
	AgentId *string `json:"agent_id"`
}

func (c *ChatModule) CreateChannel(payload *ChatChannel) (*string, error) {
	var err error
	var id string

	id = uuid.NewString()

	_, err = c.DB.Exec("INSERT INTO chat_channel (id, `name`, agent_id) VALUES (?, ?, ?);", id, payload.Name, payload.AgentId)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

func (c *ChatModule) ReadChannel(id string) (*ChatChannel, error) {
	var err error
	var rows *sql.Rows
	var channel ChatChannel

	rows, err = c.DB.Query("SELECT id, `name`, agent_id, created_at, updated_at FROM chat_channel WHERE id = ?;", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		err = fmt.Errorf("channel '%s' is not exists", id)
		return nil, err
	}

	err = rows.Scan(&channel.Id, &channel.Name, &channel.AgentId, &channel.CreatedAt, &channel.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &channel, nil
}

func (c *ChatModule) UpdateChannel(id string, payload *UpdateChatChannel) error {
	var err error
	var query string

	var sets []string = make([]string, 0)
	var args []any = make([]any, 0)

	if payload.Name != nil {
		sets = append(sets, "`name` = ?")
		args = append(args, *payload.Name)
	}

	if payload.AgentId != nil {
		sets = append(sets, "agent_id = ?")
		args = append(args, *payload.AgentId)
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)

	query = fmt.Sprintf("UPDATE chat_channel SET %s WHERE id = ?;", strings.Join(sets, ", "))
	_, err = c.DB.Exec(query, args...)
	if err != nil {
		return err
	}

	return nil
}

func (c *ChatModule) DeleteChannel(id string) error {
	var err error

	_, err = c.DB.Exec("DELETE FROM chat_channel WHERE id = ?;", id)
	if err != nil {
		return err
	}

	return nil
}
