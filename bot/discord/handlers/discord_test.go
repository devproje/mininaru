// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/bot/discord/commands"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
)

func TestInteractionUserFallsBackToGuildMember(t *testing.T) {
	var interaction *discordgo.InteractionCreate
	var found *discordgo.User

	interaction = &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Member: &discordgo.Member{User: &discordgo.User{ID: "guild-user"}},
	}}
	found = interactionUser(interaction)
	if found == nil || found.ID != "guild-user" {
		t.Fatalf("interaction user = %#v, want guild member user", found)
	}
}

func TestInteractionUserPrefersDirectUser(t *testing.T) {
	var interaction *discordgo.InteractionCreate
	var found *discordgo.User

	interaction = &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		User:   &discordgo.User{ID: "direct-user"},
		Member: &discordgo.Member{User: &discordgo.User{ID: "guild-user"}},
	}}
	found = interactionUser(interaction)
	if found == nil || found.ID != "direct-user" {
		t.Fatalf("interaction user = %#v, want direct user", found)
	}
}

func TestContextPromptsOnlySearchWithWebTool(t *testing.T) {
	var prompt string
	var defs []modules.Def

	prompt, defs = contextPrompt("Message Analyzer")
	if prompt == "" || len(defs) != 0 {
		t.Fatalf("analyzer prompt=%q tools=%d", prompt, len(defs))
	}
	prompt, defs = contextPrompt("Content Search")
	if prompt == "" || len(defs) != 1 || defs[0].Name != "web_search" {
		t.Fatalf("search prompt=%q tools=%v", prompt, defs)
	}
}

func TestCommandSetsKeepUserAppCommandsGlobal(t *testing.T) {
	var guild []*discordgo.ApplicationCommand
	var global []*discordgo.ApplicationCommand
	var command *discordgo.ApplicationCommand
	var foundAnalyzer bool
	var foundChat bool

	guild, global = commands.Sets("guild-1")
	if len(guild) == 0 {
		t.Fatal("guild commands are empty")
	}
	for _, command = range global {
		foundAnalyzer = foundAnalyzer || command.Name == "Message Analyzer"
		foundChat = foundChat || command.Name == "chat"
	}
	if !foundAnalyzer || !foundChat {
		t.Fatalf("global commands missing analyzer=%v chat=%v", foundAnalyzer, foundChat)
	}
}

func TestBotWithoutStoredIdDoesNotGrantAdminRole(t *testing.T) {
	var bot Discord
	var role string

	var err error

	bot = Discord{cfg: Config{BotId: ""}}
	role, err = bot.role("any-user")
	if err != nil {
		t.Fatal(err)
	}
	if role != "" {
		t.Fatalf("bot without stored id role = %q, want no access", role)
	}
}

func TestStopCancelsInFlightTurns(t *testing.T) {
	var bot *Discord
	var ctx context.Context
	var cancel context.CancelFunc

	var err error

	bot, err = New(Config{Token: "test-token", BotId: "bot-1"}, core.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = bot.turnContext()
	defer cancel()

	if ctx.Err() != nil {
		t.Fatalf("fresh turn context already done: %v", ctx.Err())
	}

	bot.shutdown()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the in-flight turn context")
	}
}

func TestTurnContextCarriesADeadline(t *testing.T) {
	var bot *Discord
	var ctx context.Context
	var cancel context.CancelFunc
	var deadline time.Time
	var ok bool

	var err error

	bot, err = New(Config{Token: "test-token", BotId: "bot-1"}, core.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	defer bot.shutdown()

	ctx, cancel = bot.turnContext()
	defer cancel()

	deadline, ok = ctx.Deadline()
	if !ok {
		t.Fatal("turn context has no deadline")
	}
	if time.Until(deadline) > replyTimeout+time.Second {
		t.Fatalf("deadline is %v away, want at most %v", time.Until(deadline), replyTimeout)
	}
}
