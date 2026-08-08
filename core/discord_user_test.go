package core

import (
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/util"
)

func setupDiscordUsers(t *testing.T) {
	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "discord.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { util.DB.Close() })
}

func TestDiscordPairClaimsAdminOnce(t *testing.T) {
	var code string
	var claimed bool
	var role string

	var err error

	setupDiscordUsers(t)
	code, err = DiscordPairCreate("bot-1")
	if err != nil {
		t.Fatal(err)
	}
	claimed, err = DiscordPairClaim("bot-1", code, "user-1")
	if err != nil || !claimed {
		t.Fatalf("claim = %v, %v", claimed, err)
	}
	role, err = DiscordUserRole("bot-1", "user-1")
	if err != nil || role != DiscordRoleAdmin {
		t.Fatalf("role = %q, %v", role, err)
	}
	claimed, err = DiscordPairClaim("bot-1", code, "user-2")
	if err != nil || claimed {
		t.Fatalf("reused claim = %v, %v", claimed, err)
	}
}

func TestDiscordPairCreateSweepsExpiredCodes(t *testing.T) {
	var pending int
	var claimed bool

	var err error

	setupDiscordUsers(t)

	_, err = util.DB.Exec(`INSERT INTO discord_pairings (code, bot_id, created_at)
		VALUES ('STALE001', 'bot-1', datetime('now', '-30 minutes'));`)
	if err != nil {
		t.Fatal(err)
	}

	claimed, err = DiscordPairClaim("bot-1", "STALE001", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("an expired code was claimable")
	}

	_, err = DiscordPairCreate("bot-1")
	if err != nil {
		t.Fatal(err)
	}

	err = util.DB.QueryRow("SELECT COUNT(*) FROM discord_pairings;").Scan(&pending)
	if err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("discord_pairings holds %d rows, want only the fresh one", pending)
	}
}

func TestDiscordUserAddDefaultsToScopedUser(t *testing.T) {
	var role string

	var err error

	setupDiscordUsers(t)
	err = DiscordUserAdd("bot-1", "user-1", DiscordRoleUser)
	if err != nil {
		t.Fatal(err)
	}
	role, err = DiscordUserRole("bot-1", "user-1")
	if err != nil || role != DiscordRoleUser {
		t.Fatalf("role = %q, %v", role, err)
	}
	role, err = DiscordUserRole("bot-2", "user-1")
	if err != nil || role != "" {
		t.Fatalf("other bot role = %q, %v", role, err)
	}
}
