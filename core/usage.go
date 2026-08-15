// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

type UsageLine struct {
	Kind             string `json:"kind"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

type UsageTotals struct {
	SessionId        string      `json:"session_id"`
	Lines            []UsageLine `json:"lines"`
	PromptTokens     int64       `json:"prompt_tokens"`
	CompletionTokens int64       `json:"completion_tokens"`
	TotalTokens      int64       `json:"total_tokens"`
}

const (
	UsageTurn       = "turn"
	UsageCompaction = "compaction"
	UsageSubagent   = "subagent"
)

func usageRecord(sessionId, messageId, kind string, usage openai.CompletionUsage) {
	var err error

	if sessionId == "" || usage.TotalTokens == 0 {
		return
	}

	_, err = util.DB.Exec(`INSERT INTO token_usage
		(id, session_id, message_id, kind, prompt_tokens, completion_tokens, total_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?);`,
		uuid.NewString(), sessionId, messageId, kind,
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	if err != nil {
		util.Log.Warn("recording token usage failed",
			"session", sessionId, "kind", kind, "error", err)
	}
}

func SessionUsage(sessionId string) (*UsageTotals, error) {
	var totals UsageTotals
	var rows *sql.Rows
	var line UsageLine

	var err error

	totals.SessionId = sessionId

	rows, err = util.DB.Query(`SELECT kind, SUM(prompt_tokens), SUM(completion_tokens), SUM(total_tokens)
		FROM token_usage WHERE session_id = ? GROUP BY kind ORDER BY kind ASC;`, sessionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&line.Kind, &line.PromptTokens, &line.CompletionTokens, &line.TotalTokens)
		if err != nil {
			return nil, err
		}

		totals.Lines = append(totals.Lines, line)
		totals.PromptTokens += line.PromptTokens
		totals.CompletionTokens += line.CompletionTokens
		totals.TotalTokens += line.TotalTokens
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return &totals, nil
}

func SessionUsageAll(agentId string) (map[string]int64, error) {
	var rows *sql.Rows
	var totals map[string]int64
	var sessionId string
	var total int64

	var err error

	rows, err = util.DB.Query(`SELECT u.session_id, SUM(u.total_tokens)
		FROM token_usage u JOIN sessions s ON s.id = u.session_id
		WHERE s.agent_id = ? GROUP BY u.session_id;`, agentId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals = make(map[string]int64)

	for rows.Next() {
		err = rows.Scan(&sessionId, &total)
		if err != nil {
			return nil, err
		}

		totals[sessionId] = total
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return totals, nil
}
