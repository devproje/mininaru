// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
)

type ThinkingLevel string

const (
	Off    ThinkingLevel = "off"
	Low    ThinkingLevel = "low"
	Medium ThinkingLevel = "medium"
	High   ThinkingLevel = "high"
	Max    ThinkingLevel = "max"
)

type Agent struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	Model         string `json:"model"`
	Soul          string `json:"soul"`
	ThinkingLevel string `json:"thinking_level"`
	MaxContext    uint64 `json:"max_context"`
}

func AgentCreate(agent *Agent) error {
	var opts []string
	var values []any
	var i int
	var wild []string

	var query string
	var stmt *sql.Stmt

	var err error

	opts = []string{"id", "name", "model"}
	values = []any{agent.Id, agent.Name, agent.Model}

	if agent.Soul != "" {
		opts = append(opts, "soul")
		values = append(values, agent.Soul)
	}

	if agent.ThinkingLevel != "" {
		opts = append(opts, "thinking_level")
		values = append(values, agent.ThinkingLevel)
	}

	if agent.MaxContext != 0 {
		opts = append(opts, "max_context")
		values = append(values, agent.MaxContext)
	}

	if agent.Id == "" || agent.Name == "" || agent.Model == "" {
		err = fmt.Errorf("Agent id or name, model is required")
		return err
	}

	for i = 0; i < len(opts); i++ {
		wild = append(wild, "?")
	}

	query = fmt.Sprintf("INSERT INTO agents (%s) VALUES (%s)", strings.Join(opts, ", "), strings.Join(wild, ", "))
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

func AgentRead(id string) (*Agent, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj Agent

	var err error

	stmt, err = util.DB.Prepare("SELECT id, name, model, soul, thinking_level, max_context FROM agents WHERE id = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(id)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.Name, &obj.Model, &obj.Soul, &obj.ThinkingLevel, &obj.MaxContext)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func AgentByName(name string) (*Agent, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj Agent

	var err error

	stmt, err = util.DB.Prepare("SELECT id, name, model, soul, thinking_level, max_context FROM agents WHERE name = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(name)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.Name, &obj.Model, &obj.Soul, &obj.ThinkingLevel, &obj.MaxContext)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func AgentList() ([]*Agent, error) {
	var rows *sql.Rows
	var list []*Agent
	var obj Agent

	var err error

	rows, err = util.DB.Query("SELECT id, name, model, soul, thinking_level, max_context FROM agents ORDER BY name ASC;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&obj.Id, &obj.Name, &obj.Model, &obj.Soul, &obj.ThinkingLevel, &obj.MaxContext)
		if err != nil {
			return nil, err
		}

		list = append(list, &Agent{
			Id:            obj.Id,
			Name:          obj.Name,
			Model:         obj.Model,
			Soul:          obj.Soul,
			ThinkingLevel: obj.ThinkingLevel,
			MaxContext:    obj.MaxContext,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func AgentUpdate(id string, agent *Agent) error {
	var opts []string
	var values []any
	var query string

	var stmt *sql.Stmt
	var err error

	if agent.Name != "" {
		opts = append(opts, "name = ?")
		values = append(values, agent.Name)
	}

	if agent.Model != "" {
		opts = append(opts, "model = ?")
		values = append(values, agent.Model)
	}

	if agent.Soul != "" {
		opts = append(opts, "soul = ?")
		values = append(values, agent.Soul)
	}

	if agent.ThinkingLevel != "" {
		opts = append(opts, "thinking_level = ?")
		values = append(values, agent.ThinkingLevel)
	}

	if agent.MaxContext != 0 {
		opts = append(opts, "max_context = ?")
		values = append(values, agent.MaxContext)
	}

	values = append(values, id)
	query = fmt.Sprintf("UPDATE agents SET %s WHERE id = ?;", strings.Join(opts, ", "))

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

func AgentDelete(id string) error {
	var stmt *sql.Stmt
	var err error

	stmt, err = util.DB.Prepare("DELETE FROM agents WHERE id = ?;")
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
