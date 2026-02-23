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
	"time"
)

type AgentInstruction struct {
	Id           string    `json:"id"`
	AgentId      string    `json:"agent_id"`
	Instructions string    `json:"instructions"`
	CreatedAt    time.Time `json:"created_at"`
}

func (a *AgentModule) CreateInstructions() error {
	return nil
}

func (a *AgentModule) ReadInstuctions(agentId string) error {
	return nil
}
