// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/core"
)

func (d *Discord) respond(interaction *discordgo.InteractionCreate, text string) {
	d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: text, Flags: discordgo.MessageFlagsEphemeral},
	})
}

func (d *Discord) pairCommand(interaction *discordgo.InteractionCreate, user *discordgo.User) {
	var options []*discordgo.ApplicationCommandInteractionDataOption
	var claimed bool

	var err error

	if d.cfg.BotId == "" {
		d.respond(interaction, "Pairing needs a bot configured with `mininaru bot add`.")
		return
	}
	options = interaction.ApplicationCommandData().Options
	if len(options) != 1 {
		d.respond(interaction, "Give me the pairing code from `mininaru bot pair`.")
		return
	}
	claimed, err = core.DiscordPairClaim(d.cfg.BotId, options[0].StringValue(), user.ID)
	if err != nil {
		d.respond(interaction, publicFailure("pairing you", err))
		return
	}
	if !claimed {
		d.respond(interaction, "That code is wrong, expired, or already used.")
		return
	}
	d.respond(interaction, "You are the admin now.")
}

func (d *Discord) resetCommand(interaction *discordgo.InteractionCreate) {
	var channelId string
	var target *core.Instance

	var err error

	channelId = interaction.ChannelID
	target, err = d.instance(channelId)
	if err != nil {
		d.respond(interaction, publicFailure("looking up the agent", err))
		return
	}
	_, err = core.SessionAttach(target.Agent, OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.respond(interaction, publicFailure("resetting the conversation", err))
		return
	}
	d.respond(interaction, "Fresh start with "+target.Agent.Name+". I forgot what we were talking about.")
}

func (d *Discord) agentCommand(interaction *discordgo.InteractionCreate) {
	var channelId string
	var options []*discordgo.ApplicationCommandInteractionDataOption
	var target *core.Instance
	var name string

	var err error

	channelId = interaction.ChannelID
	options = interaction.ApplicationCommandData().Options
	if len(options) == 0 {
		target, err = d.instance(channelId)
		if err != nil {
			d.respond(interaction, publicFailure("looking up the agent", err))
			return
		}
		d.respond(interaction, "This channel talks to "+target.Agent.Name+".")
		return
	}

	name = options[0].StringValue()
	target, err = d.registry.Get(name)
	if err != nil {
		publicFailure("looking up the agent", err)
		d.respond(interaction, "No agent by that name.")
		return
	}
	_, err = core.SessionAttach(target.Agent, OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.respond(interaction, publicFailure("switching the agent", err))
		return
	}
	d.respond(interaction, "This channel talks to "+name+" now, starting fresh.")
}

func (d *Discord) userCommand(interaction *discordgo.InteractionCreate) {
	var actor *discordgo.User
	var role string
	var options []*discordgo.ApplicationCommandInteractionDataOption
	var user *discordgo.User

	var err error

	actor = interactionUser(interaction)
	if actor == nil {
		d.respond(interaction, "I could not tell who asked.")
		return
	}
	role, err = d.role(actor.ID)
	if err != nil || role != core.DiscordRoleAdmin {
		d.respond(interaction, "Only the admin can do that.")
		return
	}
	options = interaction.ApplicationCommandData().Options
	if len(options) != 1 || len(options[0].Options) != 1 {
		d.respond(interaction, "Pick a user to add.")
		return
	}
	user = options[0].Options[0].UserValue(nil)
	if user == nil || user.Bot {
		d.respond(interaction, "Pick a person, not a bot.")
		return
	}
	err = core.DiscordUserAdd(d.cfg.BotId, user.ID, core.DiscordRoleUser)
	if err != nil {
		d.respond(interaction, publicFailure("adding the user", err))
		return
	}
	d.respond(interaction, "<@"+user.ID+"> can talk to me now.")
}

func (d *Discord) onInteraction(gateway *discordgo.Session, interaction *discordgo.InteractionCreate) {
	var user *discordgo.User
	var data discordgo.ApplicationCommandInteractionData
	var role string

	var err error

	if interaction.Type == discordgo.InteractionMessageComponent {
		d.approvalInteraction(interaction)
		return
	}
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}
	user = interactionUser(interaction)
	if user == nil {
		return
	}
	data = interaction.ApplicationCommandData()
	if data.CommandType == discordgo.MessageApplicationCommand {
		d.contextCommand(interaction, user)
		return
	}
	if data.Name == "pair" {
		d.pairCommand(interaction, user)
		return
	}
	if data.Name == "chat" {
		d.chatCommand(interaction, user)
		return
	}
	role, err = d.role(user.ID)
	if err != nil || role == "" {
		d.respond(interaction, "You are not paired with this bot.")
		return
	}

	switch data.Name {
	case "reset":
		d.resetCommand(interaction)
	case "agent":
		d.agentCommand(interaction)
	case "user":
		d.userCommand(interaction)
	}
}
