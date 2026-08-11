// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"
)

func TestDiscordMentionDefaultsToOff(t *testing.T) {
	var enabled bool

	var err error

	setupDiscordUsers(t)

	err = DiscordUserAdd("bot-1", "user-1", DiscordRoleUser)
	if err != nil {
		t.Fatal(err)
	}

	enabled, err = DiscordMentionEnabled("bot-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("a freshly added user may be pinged, want mentions off by default")
	}
}

func TestDiscordMentionRoundTrips(t *testing.T) {
	var enabled bool

	var err error

	setupDiscordUsers(t)

	err = DiscordUserAdd("bot-1", "user-1", DiscordRoleUser)
	if err != nil {
		t.Fatal(err)
	}

	err = DiscordMentionSet("bot-1", "user-1", true)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err = DiscordMentionEnabled("bot-1", "user-1")
	if err != nil || !enabled {
		t.Fatalf("mention enabled = %v, err = %v", enabled, err)
	}

	err = DiscordMentionSet("bot-1", "user-1", false)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err = DiscordMentionEnabled("bot-1", "user-1")
	if err != nil || enabled {
		t.Fatalf("mention enabled = %v after turning it off, err = %v", enabled, err)
	}
}

func TestDiscordMentionRejectsAnUnpairedUser(t *testing.T) {
	var err error

	setupDiscordUsers(t)

	err = DiscordMentionSet("bot-1", "stranger", true)
	if err == nil {
		t.Fatal("an unpaired user was allowed to opt into pings")
	}
}

func TestDiscordMentionUsersListsOnlyOptedInOnesForThisBot(t *testing.T) {
	var opted map[string]bool
	var pair [2]string

	var err error

	setupDiscordUsers(t)

	for _, pair = range [][2]string{{"bot-1", "yes"}, {"bot-1", "no"}, {"bot-2", "other"}} {
		err = DiscordUserAdd(pair[0], pair[1], DiscordRoleUser)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = DiscordMentionSet("bot-1", "yes", true)
	if err != nil {
		t.Fatal(err)
	}
	err = DiscordMentionSet("bot-2", "other", true)
	if err != nil {
		t.Fatal(err)
	}

	opted, err = DiscordMentionUsers("bot-1")
	if err != nil {
		t.Fatal(err)
	}
	if !opted["yes"] {
		t.Fatal("an opted-in user is missing")
	}
	if opted["no"] {
		t.Fatal("a user who never opted in is listed")
	}
	if opted["other"] {
		t.Fatal("another bot's user leaked into this bot's list")
	}
}

func TestDiscordMentionSurvivesARoleChange(t *testing.T) {
	var enabled bool

	var err error

	setupDiscordUsers(t)

	err = DiscordUserAdd("bot-1", "user-1", DiscordRoleUser)
	if err != nil {
		t.Fatal(err)
	}
	err = DiscordMentionSet("bot-1", "user-1", true)
	if err != nil {
		t.Fatal(err)
	}

	err = DiscordUserAdd("bot-1", "user-1", DiscordRoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	enabled, err = DiscordMentionEnabled("bot-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("promoting a user to admin reset their mention setting")
	}
}
