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
	CachedTokens     int64  `json:"cached_tokens"`
}

type UsageTotals struct {
	SessionId        string      `json:"session_id"`
	Lines            []UsageLine `json:"lines"`
	PromptTokens     int64       `json:"prompt_tokens"`
	CompletionTokens int64       `json:"completion_tokens"`
	TotalTokens      int64       `json:"total_tokens"`
	CachedTokens     int64       `json:"cached_tokens"`
}

const (
	UsageTurn       = "turn"
	UsageCompaction = "compaction"
	UsageSubagent   = "subagent"
)

func usageRecord(sessionId, messageId, kind string, usage openai.CompletionUsage) {
	usageRecordWithContext(sessionId, messageId, kind, usage, 0, 0)
}

func usageRecordWithContext(sessionId, messageId, kind string, usage openai.CompletionUsage, contextTokens, contextWindow int64) {
	var err error

	if sessionId == "" || usage.TotalTokens == 0 {
		return
	}

	_, err = util.DB.Exec(`INSERT INTO token_usage
		(id, session_id, message_id, kind, prompt_tokens, completion_tokens, total_tokens, context_tokens, context_window, cached_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		uuid.NewString(), sessionId, messageId, kind,
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, contextTokens, contextWindow,
		usage.PromptTokensDetails.CachedTokens)
	if err != nil {
		util.Log.Warn("recording token usage failed",
			"session", sessionId, "kind", kind, "error", err)
	}
}

func SessionContextTokens(sessionId string) (int64, int64, bool, error) {
	var tokens int64
	var window int64

	var err error

	err = util.DB.QueryRow(`SELECT u.context_tokens, u.context_window
		FROM token_usage u
		JOIN messages m ON m.id = u.message_id
		LEFT JOIN session_summaries s ON s.session_id = u.session_id
		LEFT JOIN messages compacted ON compacted.id = s.through_message_id
		WHERE u.session_id = ? AND u.kind = ?
			AND (compacted.rowid IS NULL OR m.rowid > compacted.rowid)
		ORDER BY u.rowid DESC LIMIT 1;`, sessionId, UsageTurn).Scan(&tokens, &window)
	if err != nil {
		if err == sql.ErrNoRows {
			err = util.DB.QueryRow(`SELECT context_window FROM token_usage
				WHERE session_id = ? AND kind = ? AND context_window > 0
				ORDER BY rowid DESC LIMIT 1;`, sessionId, UsageTurn).Scan(&window)
			if err == sql.ErrNoRows {
				return 0, 0, false, nil
			}
			if err != nil {
				return 0, 0, false, err
			}
			return 0, window, false, nil
		}
		return 0, 0, false, err
	}
	if tokens <= 0 {
		return 0, window, false, nil
	}

	return tokens, window, true, nil
}

func SessionUsage(sessionId string) (*UsageTotals, error) {
	var totals UsageTotals
	var rows *sql.Rows
	var line UsageLine

	var err error

	totals.SessionId = sessionId

	rows, err = util.DB.Query(`SELECT kind, SUM(prompt_tokens), SUM(completion_tokens), SUM(total_tokens), SUM(cached_tokens)
		FROM token_usage WHERE session_id = ? GROUP BY kind ORDER BY kind ASC;`, sessionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&line.Kind, &line.PromptTokens, &line.CompletionTokens, &line.TotalTokens, &line.CachedTokens)
		if err != nil {
			return nil, err
		}

		totals.Lines = append(totals.Lines, line)
		totals.PromptTokens += line.PromptTokens
		totals.CompletionTokens += line.CompletionTokens
		totals.TotalTokens += line.TotalTokens
		totals.CachedTokens += line.CachedTokens
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
