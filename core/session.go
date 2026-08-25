// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
)

type Session struct {
	Id        string `json:"id"`
	AgentId   string `json:"agent_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func SessionCreate(session *Session) error {
	var opts []string
	var values []any
	var i int
	var wild []string

	var query string
	var stmt *sql.Stmt

	var err error

	opts = []string{"id", "agent_id"}
	values = []any{session.Id, session.AgentId}

	if session.Name != "" {
		opts = append(opts, "name")
		values = append(values, session.Name)
	}

	if session.Id == "" || session.AgentId == "" {
		err = fmt.Errorf("session id or agent_id is required")
		return err
	}

	for i = 0; i < len(opts); i++ {
		wild = append(wild, "?")
	}

	query = fmt.Sprintf("INSERT INTO sessions (%s) VALUES (%s);", strings.Join(opts, ", "), strings.Join(wild, ", "))

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

func SessionRead(id string) (*Session, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj Session

	var err error

	stmt, err = util.DB.Prepare("SELECT id, agent_id, name, created_at FROM sessions WHERE id = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(id)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.AgentId, &obj.Name, &obj.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func SessionList(agentId string) ([]*Session, error) {
	var stmt *sql.Stmt
	var rows *sql.Rows
	var list []*Session
	var obj Session

	var err error

	stmt, err = util.DB.Prepare("SELECT id, agent_id, name, created_at FROM sessions WHERE agent_id = ? ORDER BY created_at DESC;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err = stmt.Query(agentId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&obj.Id, &obj.AgentId, &obj.Name, &obj.CreatedAt)
		if err != nil {
			return nil, err
		}

		list = append(list, &Session{Id: obj.Id, AgentId: obj.AgentId, Name: obj.Name, CreatedAt: obj.CreatedAt})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func SessionUpdate(id string, session *Session) error {
	var opts []string
	var values []any
	var query string

	var stmt *sql.Stmt
	var err error

	if session.Name != "" {
		opts = append(opts, "name = ?")
		values = append(values, session.Name)
	}

	values = append(values, id)
	query = fmt.Sprintf("UPDATE sessions SET %s WHERE id = ?;", strings.Join(opts, ", "))

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

func SessionDelete(id string) error {
	var stmt *sql.Stmt
	var err error

	stmt, err = util.DB.Prepare("DELETE FROM sessions WHERE id = ?;")
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
