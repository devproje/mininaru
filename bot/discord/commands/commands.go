// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import "github.com/bwmarrin/discordgo"

var userInstallTypes = []discordgo.ApplicationIntegrationType{discordgo.ApplicationIntegrationUserInstall}

var userInstallContexts = []discordgo.InteractionContextType{
	discordgo.InteractionContextGuild,
	discordgo.InteractionContextBotDM,
	discordgo.InteractionContextPrivateChannel,
}

var slashCommands = []*discordgo.ApplicationCommand{
	{
		Name: "pair", Description: "Become this bot's admin",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "code", Description: "The one-time code from `mininaru bot pair`", Required: true},
		},
	},
	{Name: "reset", Description: "Forget this channel's conversation and start over"},
	{Name: "compact", Description: "Fold this channel's conversation into a summary (admin only)"},
	{Name: "usage", Description: "Show the tokens this channel's conversation has spent (admin only)"},
	{
		Name: "agent", Description: "Show or switch who answers in this channel",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type: discordgo.ApplicationCommandOptionString, Name: "name",
				Description: "Agent to switch to, or leave empty to see the current one", Required: false, Autocomplete: true,
			},
		},
	},
	{
		Name: "mention", Description: "Show or set whether this bot may ping you",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type: discordgo.ApplicationCommandOptionString, Name: "state",
				Description: "Leave empty to see the current setting", Required: false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "on", Value: "on"},
					{Name: "off", Value: "off"},
				},
			},
		},
	},
	{
		Name: "user", Description: "Manage who is allowed to talk to this bot",
		Options: []*discordgo.ApplicationCommandOption{{
			Type: discordgo.ApplicationCommandOptionSubCommand, Name: "add", Description: "Let someone talk to this bot",
			Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Person to allow", Required: true}},
		}},
	},
	{
		Name: "chat", Description: "Ask one question, with nothing remembered afterwards",
		IntegrationTypes: &userInstallTypes, Contexts: &userInstallContexts,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "content", Description: "What to ask", Required: true},
			{Type: discordgo.ApplicationCommandOptionAttachment, Name: "attachment", Description: "A file or image to include", Required: false},
			{Type: discordgo.ApplicationCommandOptionBoolean, Name: "ephemeral", Description: "Keep the reply private, on by default", Required: false},
		},
	},
}

var userAppCommands = []*discordgo.ApplicationCommand{
	{Name: "Message Analyzer", Type: discordgo.MessageApplicationCommand, IntegrationTypes: &userInstallTypes, Contexts: &userInstallContexts},
	{Name: "Content Search", Type: discordgo.MessageApplicationCommand, IntegrationTypes: &userInstallTypes, Contexts: &userInstallContexts},
}

func Sets(guildId string) ([]*discordgo.ApplicationCommand, []*discordgo.ApplicationCommand) {
	var global []*discordgo.ApplicationCommand
	var guild []*discordgo.ApplicationCommand
	var command *discordgo.ApplicationCommand

	if guildId == "" {
		global = append(global, slashCommands...)
		global = append(global, userAppCommands...)
		return guild, global
	}
	for _, command = range slashCommands {
		if command.Name == "chat" {
			global = append(global, command)
			continue
		}
		guild = append(guild, command)
	}
	global = append(global, userAppCommands...)
	return guild, global
}
