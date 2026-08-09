// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
)

const (
	DiscordRoleUser  = "user"
	DiscordRoleAdmin = "admin"
)

const discordPairWindow = "-10 minutes"

func discordPairSweep(exec func(string, ...any) (sql.Result, error)) error {
	var err error

	_, err = exec("DELETE FROM discord_pairings WHERE created_at < datetime('now', ?);", discordPairWindow)

	return err
}

func DiscordUserRole(botId, userId string) (string, error) {
	var role string

	var err error

	err = util.DB.QueryRow("SELECT role FROM discord_users WHERE bot_id = ? AND user_id = ?;", botId, userId).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

func DiscordUserAdd(botId, userId, role string) error {
	var err error

	if botId == "" || userId == "" {
		return fmt.Errorf("bot and user ids are required")
	}
	if role != DiscordRoleUser && role != DiscordRoleAdmin {
		return fmt.Errorf("invalid discord role %q", role)
	}

	_, err = util.DB.Exec(`INSERT INTO discord_users (bot_id, user_id, role) VALUES (?, ?, ?)
		ON CONFLICT(bot_id, user_id) DO UPDATE SET role = excluded.role;`, botId, userId, role)
	return err
}

func DiscordMentionEnabled(botId, userId string) (bool, error) {
	var enabled bool

	var err error

	err = util.DB.QueryRow("SELECT mention_enabled FROM discord_users WHERE bot_id = ? AND user_id = ?;",
		botId, userId).Scan(&enabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return enabled, err
}

func DiscordMentionSet(botId, userId string, enabled bool) error {
	var result sql.Result
	var changed int64

	var err error

	if botId == "" || userId == "" {
		return fmt.Errorf("bot and user ids are required")
	}

	result, err = util.DB.Exec("UPDATE discord_users SET mention_enabled = ? WHERE bot_id = ? AND user_id = ?;",
		enabled, botId, userId)
	if err != nil {
		return err
	}

	changed, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("discord user is not paired with this bot")
	}

	return nil
}

func DiscordMentionUsers(botId string) (map[string]bool, error) {
	var rows *sql.Rows
	var allowed map[string]bool
	var userId string

	var err error

	allowed = make(map[string]bool)

	rows, err = util.DB.Query("SELECT user_id FROM discord_users WHERE bot_id = ? AND mention_enabled = 1;", botId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&userId)
		if err != nil {
			return nil, err
		}
		allowed[userId] = true
	}

	return allowed, rows.Err()
}

func DiscordPairCreate(botId string) (string, error) {
	var raw [8]byte
	var index int
	var code strings.Builder

	var err error

	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

	if botId == "" {
		return "", fmt.Errorf("bot id is required")
	}
	_, err = rand.Read(raw[:])
	if err != nil {
		return "", err
	}
	for index = range raw {
		code.WriteByte(alphabet[int(raw[index])%len(alphabet)])
	}

	err = discordPairSweep(util.DB.Exec)
	if err != nil {
		return "", err
	}

	_, err = util.DB.Exec("INSERT INTO discord_pairings (code, bot_id) VALUES (?, ?);", code.String(), botId)
	if err != nil {
		return "", err
	}
	return code.String(), nil
}

func DiscordPairClaim(botId, code, userId string) (bool, error) {
	var tx *sql.Tx
	var found string

	var err error

	tx, err = util.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	err = discordPairSweep(tx.Exec)
	if err != nil {
		return false, err
	}

	err = tx.QueryRow("SELECT bot_id FROM discord_pairings WHERE code = ? AND created_at >= datetime('now', ?);",
		strings.ToUpper(code), discordPairWindow).Scan(&found)
	if err == sql.ErrNoRows || found != botId {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(`INSERT INTO discord_users (bot_id, user_id, role) VALUES (?, ?, ?)
		ON CONFLICT(bot_id, user_id) DO UPDATE SET role = excluded.role;`, botId, userId, DiscordRoleAdmin)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec("DELETE FROM discord_pairings WHERE code = ?;", strings.ToUpper(code))
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
