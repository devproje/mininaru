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
