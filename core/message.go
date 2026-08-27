// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
)

type Message struct {
	Id        string `json:"id"`
	SessionId string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	CreatedAt string `json:"created_at"`
}

func MessageCreate(msg *Message) error {
	var opts []string
	var values []any
	var i int
	var wild []string

	var query string
	var stmt *sql.Stmt

	var err error

	opts = []string{"id", "session_id", "role", "content"}
	values = []any{msg.Id, msg.SessionId, msg.Role, msg.Content}

	if msg.Status != "" {
		opts = append(opts, "status")
		values = append(values, msg.Status)
	}

	if msg.Error != "" {
		opts = append(opts, "error")
		values = append(values, msg.Error)
	}

	if msg.Id == "" || msg.SessionId == "" || msg.Role == "" || msg.Content == "" {
		err = fmt.Errorf("message id, session_id, role or content is required")
		return err
	}

	for i = 0; i < len(opts); i++ {
		wild = append(wild, "?")
	}

	query = fmt.Sprintf("INSERT INTO messages (%s) VALUES (%s);", strings.Join(opts, ", "), strings.Join(wild, ", "))

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

func MessageRead(id string) (*Message, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj Message

	var err error

	stmt, err = util.DB.Prepare("SELECT id, session_id, role, content, status, error, created_at FROM messages WHERE id = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(id)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.SessionId, &obj.Role, &obj.Content, &obj.Status, &obj.Error, &obj.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func MessageList(sessionId string) ([]*Message, error) {
	var stmt *sql.Stmt
	var rows *sql.Rows
	var list []*Message
	var obj Message

	var err error

	stmt, err = util.DB.Prepare("SELECT id, session_id, role, content, status, error, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err = stmt.Query(sessionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&obj.Id, &obj.SessionId, &obj.Role, &obj.Content, &obj.Status, &obj.Error, &obj.CreatedAt)
		if err != nil {
			return nil, err
		}

		list = append(list, &Message{
			Id:        obj.Id,
			SessionId: obj.SessionId,
			Role:      obj.Role,
			Content:   obj.Content,
			Status:    obj.Status,
			Error:     obj.Error,
			CreatedAt: obj.CreatedAt,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func MessageUpdate(id string, msg *Message) error {
	var opts []string
	var values []any
	var query string

	var stmt *sql.Stmt
	var err error

	if msg.Content != "" {
		opts = append(opts, "content = ?")
		values = append(values, msg.Content)
	}

	if msg.Status != "" {
		opts = append(opts, "status = ?")
		values = append(values, msg.Status)
	}

	if msg.Error != "" {
		opts = append(opts, "error = ?")
		values = append(values, msg.Error)
	}

	values = append(values, id)
	query = fmt.Sprintf("UPDATE messages SET %s WHERE id = ?;", strings.Join(opts, ", "))

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

func MessageDelete(id string) error {
	var stmt *sql.Stmt
	var err error

	stmt, err = util.DB.Prepare("DELETE FROM messages WHERE id = ?;")
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
