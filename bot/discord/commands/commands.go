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
		Name: "pair", Description: "pair yourself as this bot's admin",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "code", Description: "one-time pairing code from the CLI", Required: true},
		},
	},
	{Name: "reset", Description: "start a fresh conversation in this channel"},
	{
		Name: "agent", Description: "show or switch the agent answering in this channel",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "agent name", Required: false},
		},
	},
	{
		Name: "user", Description: "manage users allowed to talk to this bot",
		Options: []*discordgo.ApplicationCommandOption{{
			Type: discordgo.ApplicationCommandOptionSubCommand, Name: "add", Description: "add a regular user",
			Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "user to add", Required: true}},
		}},
	},
	{
		Name: "chat", Description: "run a stateless one-turn chat",
		IntegrationTypes: &userInstallTypes, Contexts: &userInstallContexts,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "content", Description: "message to send", Required: true},
			{Type: discordgo.ApplicationCommandOptionAttachment, Name: "attachment", Description: "optional file or image", Required: false},
			{Type: discordgo.ApplicationCommandOptionBoolean, Name: "ephemeral", Description: "show only to you; defaults to true", Required: false},
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
