// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"errors"

	"github.com/devproje/mininaru/util"
)

type Summary struct {
	SessionId        string `json:"session_id"`
	Content          string `json:"content"`
	ThroughMessageId string `json:"through_message_id"`
	UpdatedAt        string `json:"updated_at"`
}

func SummaryLoad(sessionId string) (*Summary, error) {
	var obj Summary

	var err error

	err = util.DB.QueryRow("SELECT session_id, content, through_message_id, updated_at FROM session_summaries WHERE session_id = ?;", sessionId).
		Scan(&obj.SessionId, &obj.Content, &obj.ThroughMessageId, &obj.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func SummarySave(summary *Summary) error {
	var err error

	_, err = util.DB.Exec(`INSERT INTO session_summaries (session_id, content, through_message_id) VALUES (?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET content = excluded.content, through_message_id = excluded.through_message_id,
		updated_at = CURRENT_TIMESTAMP;`, summary.SessionId, summary.Content, summary.ThroughMessageId)

	return err
}
