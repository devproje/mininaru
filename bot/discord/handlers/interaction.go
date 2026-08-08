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
		d.respond(interaction, "pairing requires a configured bot")
		return
	}
	options = interaction.ApplicationCommandData().Options
	if len(options) != 1 {
		d.respond(interaction, "pairing code is required")
		return
	}
	claimed, err = core.DiscordPairClaim(d.cfg.BotId, options[0].StringValue(), user.ID)
	if err != nil {
		d.respond(interaction, publicFailure("pairing", err))
		return
	}
	if !claimed {
		d.respond(interaction, "invalid, expired, or already used pairing code")
		return
	}
	d.respond(interaction, "paired as admin")
}

func (d *Discord) resetCommand(interaction *discordgo.InteractionCreate) {
	var channelId string
	var target *core.Instance

	var err error

	channelId = interaction.ChannelID
	target, err = d.instance(channelId)
	if err != nil {
		d.respond(interaction, publicFailure("agent lookup", err))
		return
	}
	_, err = core.SessionAttach(target.Agent, OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.respond(interaction, publicFailure("session reset", err))
		return
	}
	d.respond(interaction, "started a fresh conversation with "+target.Agent.Name)
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
			d.respond(interaction, publicFailure("agent lookup", err))
			return
		}
		d.respond(interaction, "this channel talks to "+target.Agent.Name)
		return
	}

	name = options[0].StringValue()
	target, err = d.registry.Get(name)
	if err != nil {
		publicFailure("agent lookup", err)
		d.respond(interaction, "agent not found")
		return
	}
	_, err = core.SessionAttach(target.Agent, OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.respond(interaction, publicFailure("agent switch", err))
		return
	}
	d.respond(interaction, "this channel now talks to "+name+", starting a fresh conversation")
}

func (d *Discord) userCommand(interaction *discordgo.InteractionCreate) {
	var actor *discordgo.User
	var role string
	var options []*discordgo.ApplicationCommandInteractionDataOption
	var user *discordgo.User

	var err error

	actor = interactionUser(interaction)
	if actor == nil {
		d.respond(interaction, "could not identify the requesting user")
		return
	}
	role, err = d.role(actor.ID)
	if err != nil || role != core.DiscordRoleAdmin {
		d.respond(interaction, "admin role is required")
		return
	}
	options = interaction.ApplicationCommandData().Options
	if len(options) != 1 || len(options[0].Options) != 1 {
		d.respond(interaction, "user is required")
		return
	}
	user = options[0].Options[0].UserValue(nil)
	if user == nil || user.Bot {
		d.respond(interaction, "select a non-bot user")
		return
	}
	err = core.DiscordUserAdd(d.cfg.BotId, user.ID, core.DiscordRoleUser)
	if err != nil {
		d.respond(interaction, publicFailure("user add", err))
		return
	}
	d.respond(interaction, "added <@"+user.ID+"> with the user role")
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
		d.respond(interaction, "you are not paired with this bot")
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
