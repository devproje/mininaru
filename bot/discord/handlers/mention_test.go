// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

func setupMentions(t *testing.T) {
	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "mention.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { util.DB.Close() })
}

func TestMentionedIdsFindsBothMentionForms(t *testing.T) {
	var ids []string

	ids = mentionedIds("hey <@111> and <@!222>, also <@111> again")

	if len(ids) != 2 || ids[0] != "111" || ids[1] != "222" {
		t.Fatalf("mentioned ids = %v, want 111 and 222 once each", ids)
	}
}

func TestMentionedIdsIgnoresRolesAndEveryone(t *testing.T) {
	var ids []string

	ids = mentionedIds("@everyone @here <@&999> ping")

	if len(ids) != 0 {
		t.Fatalf("mentioned ids = %v, want none: roles and everyone are not user mentions", ids)
	}
}

func TestSilentMentionsBlocksEverything(t *testing.T) {
	var allowed *discordgo.MessageAllowedMentions

	allowed = silentMentions()

	if allowed.Parse == nil {
		t.Fatal("Parse is nil, which lets Discord fall back to parsing every mention")
	}
	if len(allowed.Parse) != 0 || len(allowed.Users) != 0 || len(allowed.Roles) != 0 {
		t.Fatalf("silent mentions still allow something: %#v", allowed)
	}
}

func TestAllowedMentionsPingsOnlyOptedInUsers(t *testing.T) {
	var bot Discord
	var allowed *discordgo.MessageAllowedMentions

	allowed = silentMentions()
	var id string

	var err error

	setupMentions(t)

	bot = Discord{cfg: Config{BotId: "bot-1"}}

	for _, id = range []string{"111", "222"} {
		err = core.DiscordUserAdd("bot-1", id, core.DiscordRoleUser)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = core.DiscordMentionSet("bot-1", "111", true)
	if err != nil {
		t.Fatal(err)
	}

	allowed = bot.allowedMentions("<@111> and <@222> and @everyone")

	if len(allowed.Parse) != 0 {
		t.Fatalf("everyone or here could still fire: %#v", allowed)
	}
	if len(allowed.Users) != 1 || allowed.Users[0] != "111" {
		t.Fatalf("allowed users = %v, want only the opted-in one", allowed.Users)
	}
}

func TestAllowedMentionsStaySilentWithoutOptIn(t *testing.T) {
	var bot Discord
	var allowed *discordgo.MessageAllowedMentions

	allowed = silentMentions()

	var err error

	setupMentions(t)

	bot = Discord{cfg: Config{BotId: "bot-1"}}

	err = core.DiscordUserAdd("bot-1", "222", core.DiscordRoleUser)
	if err != nil {
		t.Fatal(err)
	}

	allowed = bot.allowedMentions("<@222> hello")

	if len(allowed.Users) != 0 {
		t.Fatalf("allowed users = %v, want nobody pinged", allowed.Users)
	}
}
