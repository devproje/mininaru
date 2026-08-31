// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/devproje/mininaru/util"
)

type Attachment struct {
	Id        string `json:"id"`
	SessionId string `json:"session_id"`
	MessageId string `json:"message_id"`
	Mime      string `json:"mime"`
	Bytes     int64  `json:"bytes"`
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
}

func scanAttachment(scanner interface{ Scan(...any) error }) (*Attachment, error) {
	var obj Attachment
	var messageId sql.NullString

	var err error

	err = scanner.Scan(&obj.Id, &obj.SessionId, &messageId, &obj.Mime, &obj.Bytes, &obj.Path, &obj.CreatedAt)
	if err != nil {
		return nil, err
	}

	obj.MessageId = messageId.String

	return &obj, nil
}

func AttachmentCreate(att *Attachment) error {
	var err error

	if att.Id == "" || att.SessionId == "" || att.Mime == "" || att.Path == "" {
		return fmt.Errorf("attachment id, session_id, mime and path are required")
	}

	if !strings.HasPrefix(att.Mime, "image/") {
		return fmt.Errorf("only image attachments are supported, got %q", att.Mime)
	}

	_, err = util.DB.Exec(
		"INSERT INTO attachments (id, session_id, mime, bytes, path) VALUES (?, ?, ?, ?, ?);",
		att.Id, att.SessionId, att.Mime, att.Bytes, att.Path)

	return err
}

func AttachmentRead(id string) (*Attachment, error) {
	return scanAttachment(util.DB.QueryRow(
		"SELECT id, session_id, message_id, mime, bytes, path, created_at FROM attachments WHERE id = ?;", id))
}

func AttachmentList(messageId string) ([]*Attachment, error) {
	var rows *sql.Rows
	var att *Attachment
	var list []*Attachment

	var err error

	rows, err = util.DB.Query(
		"SELECT id, session_id, message_id, mime, bytes, path, created_at FROM attachments WHERE message_id = ? ORDER BY created_at ASC;",
		messageId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		att, err = scanAttachment(rows)
		if err != nil {
			return nil, err
		}

		list = append(list, att)
	}

	return list, rows.Err()
}

func AttachmentBindMessage(sessionId string, messageId string, ids []string) error {
	var id string

	var err error

	for _, id = range ids {
		_, err = util.DB.Exec(
			"UPDATE attachments SET message_id = ? WHERE id = ? AND session_id = ? AND message_id IS NULL;",
			messageId, id, sessionId)
		if err != nil {
			return err
		}
	}

	return nil
}

func AttachmentDelete(id string) error {
	var att *Attachment

	var err error

	att, err = AttachmentRead(id)
	if err != nil {
		return err
	}

	_, err = util.DB.Exec("DELETE FROM attachments WHERE id = ?;", id)
	if err != nil {
		return err
	}

	os.Remove(att.Path)

	return nil
}

func messageImages(messageId string) ([]string, error) {
	var list []*Attachment
	var att *Attachment
	var buf []byte
	var out []string

	var err error

	list, err = AttachmentList(messageId)
	if err != nil {
		return nil, err
	}

	for _, att = range list {
		buf, err = os.ReadFile(att.Path)
		if err != nil {
			return nil, err
		}

		out = append(out, "data:"+att.Mime+";base64,"+base64.StdEncoding.EncodeToString(buf))
	}

	return out, nil
}
