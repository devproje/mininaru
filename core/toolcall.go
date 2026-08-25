// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
)

type ToolCall struct {
	Id        string `json:"id"`
	MessageId string `json:"message_id"`
	CallId    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	CreatedAt string `json:"created_at"`
}

func ToolCallCreate(call *ToolCall) error {
	var opts []string
	var values []any
	var i int
	var wild []string

	var query string
	var stmt *sql.Stmt

	var err error

	opts = []string{"id", "message_id", "call_id", "name", "arguments"}
	values = []any{call.Id, call.MessageId, call.CallId, call.Name, call.Arguments}

	if call.Status != "" {
		opts = append(opts, "status")
		values = append(values, call.Status)
	}

	if call.Id == "" || call.MessageId == "" || call.CallId == "" || call.Name == "" {
		err = fmt.Errorf("tool call id, message_id, call_id or name is required")
		return err
	}

	for i = 0; i < len(opts); i++ {
		wild = append(wild, "?")
	}

	query = fmt.Sprintf("INSERT INTO tool_calls (%s) VALUES (%s);", strings.Join(opts, ", "), strings.Join(wild, ", "))

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

func ToolCallRead(id string) (*ToolCall, error) {
	var stmt *sql.Stmt
	var row *sql.Row
	var obj ToolCall

	var err error

	stmt, err = util.DB.Prepare("SELECT id, message_id, call_id, name, arguments, result, status, error, created_at FROM tool_calls WHERE id = ?;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row = stmt.QueryRow(id)
	err = row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&obj.Id, &obj.MessageId, &obj.CallId, &obj.Name, &obj.Arguments, &obj.Result, &obj.Status, &obj.Error, &obj.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func ToolCallList(messageId string) ([]*ToolCall, error) {
	var stmt *sql.Stmt
	var rows *sql.Rows
	var list []*ToolCall
	var obj ToolCall

	var err error

	stmt, err = util.DB.Prepare("SELECT id, message_id, call_id, name, arguments, result, status, error, created_at FROM tool_calls WHERE message_id = ? ORDER BY created_at ASC;")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err = stmt.Query(messageId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&obj.Id, &obj.MessageId, &obj.CallId, &obj.Name, &obj.Arguments, &obj.Result, &obj.Status, &obj.Error, &obj.CreatedAt)
		if err != nil {
			return nil, err
		}

		list = append(list, &ToolCall{
			Id:        obj.Id,
			MessageId: obj.MessageId,
			CallId:    obj.CallId,
			Name:      obj.Name,
			Arguments: obj.Arguments,
			Result:    obj.Result,
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

func ToolCallUpdate(id string, call *ToolCall) error {
	var opts []string
	var values []any
	var query string

	var stmt *sql.Stmt
	var err error

	if call.Result != "" {
		opts = append(opts, "result = ?")
		values = append(values, call.Result)
	}

	if call.Status != "" {
		opts = append(opts, "status = ?")
		values = append(values, call.Status)
	}

	if call.Error != "" {
		opts = append(opts, "error = ?")
		values = append(values, call.Error)
	}

	values = append(values, id)
	query = fmt.Sprintf("UPDATE tool_calls SET %s WHERE id = ?;", strings.Join(opts, ", "))

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
