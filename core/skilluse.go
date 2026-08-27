// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/devproje/mininaru/modules/skill"
	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type SkillUse struct {
	Skill    string `json:"skill"`
	Scope    string `json:"scope"`
	Path     string `json:"path"`
	Count    int    `json:"count"`
	LastUsed string `json:"last_used"`
}

func skillUseRecord(sessionId string, record *ToolCall) {
	var payload struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	var entry *skill.Skill
	var scope string
	var path string

	var err error

	if util.DB == nil {
		return
	}

	err = json.Unmarshal([]byte(record.Arguments), &payload)
	if err != nil {
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return
	}

	entry = skill.Find(payload.Name)
	if entry != nil {
		scope = entry.Scope
		path = entry.Path
	}

	_, err = util.DB.Exec(`INSERT INTO skill_uses (id, skill, scope, path, rel, session_id, call_id)
		VALUES (?, ?, ?, ?, ?, ?, ?);`,
		uuid.NewString(), payload.Name, scope, path, strings.TrimSpace(payload.Path), sessionId, record.CallId)
	if err != nil {
		util.Log.Warn("recording a skill use failed", "skill", payload.Name, "error", err)
	}
}

func SkillUseStats(sessionId string) ([]*SkillUse, error) {
	var query string
	var rows *sql.Rows
	var uses []*SkillUse
	var use SkillUse

	var err error

	if util.DB == nil {
		return uses, nil
	}

	query = `SELECT skill, scope, path, COUNT(*), MAX(created_at) FROM skill_uses
		WHERE (? = '' OR session_id = ?)
		GROUP BY skill, scope, path
		ORDER BY COUNT(*) DESC, MAX(created_at) DESC;`

	rows, err = util.DB.Query(query, sessionId, sessionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&use.Skill, &use.Scope, &use.Path, &use.Count, &use.LastUsed)
		if err != nil {
			return nil, err
		}

		uses = append(uses, &SkillUse{
			Skill: use.Skill, Scope: use.Scope, Path: use.Path, Count: use.Count, LastUsed: use.LastUsed,
		})
	}

	return uses, rows.Err()
}
