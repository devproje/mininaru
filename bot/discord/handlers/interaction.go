// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/core"
)

const autocompleteLimit = 25
const autocompleteNameLimit = 100

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

func (d *Discord) resetCommand(interaction *discordgo.InteractionCreate, user *discordgo.User) {
	var target *core.Instance
	var components []discordgo.MessageComponent

	var err error

	target, err = d.instance(interaction.ChannelID)
	if err != nil {
		d.respond(interaction, publicFailure("looking up the agent", err))
		return
	}
	components = resetConfirmationComponents(target.Agent.Name, user.ID)
	d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral, Components: components,
			AllowedMentions: silentMentions(),
		},
	})
}

func compactOutcome(compacted bool, err error) string {
	if err != nil {
		return publicFailure("compacting the conversation", err)
	}

	if !compacted {
		return "There is nothing to compact in this channel yet."
	}

	return "Done. This channel's conversation is a summary from here on."
}

func (d *Discord) runCompact(interaction *discordgo.InteractionCreate, agent *core.NaruAgent, session *core.Session) {
	var ctx context.Context
	var cancel context.CancelFunc
	var compacted bool
	var outcome string

	var err error

	ctx, cancel = d.turnContext()
	defer cancel()

	compacted, err = core.CompactNow(ctx, agent, session)
	outcome = compactOutcome(compacted, err)

	d.gateway.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{Content: &outcome})
}

func (d *Discord) compactCommand(interaction *discordgo.InteractionCreate, role string) {
	var bound *core.Session
	var target *core.Instance

	var err error

	if role != core.DiscordRoleAdmin {
		d.respond(interaction, "Only the admin can do that.")
		return
	}

	bound, err = core.SessionByExternal(OriginDiscord, interaction.ChannelID)
	if err != nil {
		d.respond(interaction, publicFailure("looking up this channel's conversation", err))
		return
	}
	if bound == nil {
		d.respond(interaction, "There is no conversation in this channel yet.")
		return
	}

	target, err = d.instance(interaction.ChannelID)
	if err != nil {
		d.respond(interaction, publicFailure("looking up the agent", err))
		return
	}

	err = d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: "Compacting…", Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		return
	}

	go d.runCompact(interaction, target.Agent, bound)
}

func usageReport(totals *core.UsageTotals) string {
	var builder strings.Builder
	var index int

	if totals.TotalTokens == 0 {
		return "Nothing recorded for this channel's conversation yet."
	}

	builder.WriteString("```\nKIND         PROMPT  COMPLETION       TOTAL\n")

	for index = range totals.Lines {
		fmt.Fprintf(&builder, "%-10s %8d    %8d    %8d\n", totals.Lines[index].Kind,
			totals.Lines[index].PromptTokens, totals.Lines[index].CompletionTokens,
			totals.Lines[index].TotalTokens)
	}

	fmt.Fprintf(&builder, "%-10s %8d    %8d    %8d\n```", "total",
		totals.PromptTokens, totals.CompletionTokens, totals.TotalTokens)
	builder.WriteString("\nTokens, not money — mininaru does not know what your provider charges.")

	return builder.String()
}

func (d *Discord) usageCommand(interaction *discordgo.InteractionCreate, role string) {
	var bound *core.Session
	var totals *core.UsageTotals

	var err error

	if role != core.DiscordRoleAdmin {
		d.respond(interaction, "Only the admin can do that.")
		return
	}

	bound, err = core.SessionByExternal(OriginDiscord, interaction.ChannelID)
	if err != nil {
		d.respond(interaction, publicFailure("looking up this channel's conversation", err))
		return
	}
	if bound == nil {
		d.respond(interaction, "There is no conversation in this channel yet.")
		return
	}

	totals, err = core.SessionUsage(bound.Id)
	if err != nil {
		d.respond(interaction, publicFailure("reading the token usage", err))
		return
	}

	d.respond(interaction, usageReport(totals))
}

func resetConfirmationComponents(agentName, userId string) []discordgo.MessageComponent {
	var shownName string

	shownName = strings.ReplaceAll(agentName, "`", "ˋ")
	return v2Container(approvalAccent,
		discordgo.TextDisplay{Content: "### Start a fresh conversation?\nThis will forget the current conversation with `" + shownName + "` in this channel. This cannot be undone."},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "Start fresh", Style: discordgo.DangerButton, CustomID: "reset:yes:" + userId},
			discordgo.Button{Label: "Keep conversation", Style: discordgo.SecondaryButton, CustomID: "reset:no:" + userId},
		}},
	)
}

func (d *Discord) resetInteraction(interaction *discordgo.InteractionCreate) {
	var customId string
	var parts []string
	var user *discordgo.User
	var role string
	var target *core.Instance
	var components []discordgo.MessageComponent

	var err error

	customId = interaction.MessageComponentData().CustomID
	parts = strings.Split(customId, ":")
	user = interactionUser(interaction)
	if len(parts) != 3 || user == nil || parts[2] != user.ID {
		d.respond(interaction, "That confirmation is not yours.")
		return
	}
	if parts[1] == "no" {
		components = v2Container(statusAccent, discordgo.TextDisplay{Content: "↩️ **Conversation kept**\nNothing was changed."})
		d.updateInteraction(interaction, components)
		return
	}
	if parts[1] != "yes" {
		d.respond(interaction, "That confirmation is invalid or expired.")
		return
	}
	role, err = d.role(user.ID)
	if err != nil || role == "" {
		d.respond(interaction, "You are not paired with this bot.")
		return
	}
	target, err = d.instance(interaction.ChannelID)
	if err != nil {
		d.respond(interaction, publicFailure("looking up the agent", err))
		return
	}
	_, err = core.SessionAttach(target.Agent, OriginDiscord, interaction.ChannelID, "discord "+interaction.ChannelID)
	if err != nil {
		d.respond(interaction, publicFailure("resetting the conversation", err))
		return
	}
	components = v2Container(statusAccent,
		discordgo.TextDisplay{Content: "✅ **Fresh conversation started**\nThis channel now has a clean context with **" + target.Agent.Name + "**."})
	d.updateInteraction(interaction, components)
}

func (d *Discord) updateInteraction(interaction *discordgo.InteractionCreate, components []discordgo.MessageComponent) {
	d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsIsComponentsV2, Components: components, AllowedMentions: silentMentions(),
		},
	})
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

func (d *Discord) agentAutocomplete(interaction *discordgo.InteractionCreate) {
	var data discordgo.ApplicationCommandInteractionData
	var user *discordgo.User
	var role string
	var query string
	var choices []*discordgo.ApplicationCommandOptionChoice

	var err error

	data = interaction.ApplicationCommandData()
	user = interactionUser(interaction)
	if user != nil {
		role, err = d.role(user.ID)
	}
	if err == nil && role != "" && data.Name == "agent" && len(data.Options) > 0 {
		query = data.Options[0].StringValue()
		choices = agentChoices(d.registry.List(), query)
	}
	d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func agentChoices(instances []*core.Instance, query string) []*discordgo.ApplicationCommandOptionChoice {
	var normalized string
	var instance *core.Instance
	var name string
	var choices []*discordgo.ApplicationCommandOptionChoice

	normalized = strings.ToLower(strings.TrimSpace(query))
	for _, instance = range instances {
		if instance == nil || instance.Agent == nil {
			continue
		}
		name = instance.Agent.Name
		if len([]rune(name)) > autocompleteNameLimit {
			continue
		}
		if normalized != "" && !strings.Contains(strings.ToLower(name), normalized) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: name, Value: name})
		if len(choices) == autocompleteLimit {
			break
		}
	}
	return choices
}

func (d *Discord) mentionCommand(interaction *discordgo.InteractionCreate, user *discordgo.User) {
	var options []*discordgo.ApplicationCommandInteractionDataOption
	var enabled bool

	var err error

	options = interaction.ApplicationCommandData().Options
	if len(options) == 0 {
		enabled, err = core.DiscordMentionEnabled(d.cfg.BotId, user.ID)
		if err != nil {
			d.respond(interaction, publicFailure("reading your mention setting", err))
			return
		}
		if enabled {
			d.respond(interaction, "I will ping you when I mention you.")
			return
		}
		d.respond(interaction, "I mention you without pinging. Use `/mention on` to be pinged.")
		return
	}

	enabled = options[0].StringValue() == "on"

	err = core.DiscordMentionSet(d.cfg.BotId, user.ID, enabled)
	if err != nil {
		d.respond(interaction, publicFailure("saving your mention setting", err))
		return
	}

	if enabled {
		d.respond(interaction, "I will ping you from now on.")
		return
	}
	d.respond(interaction, "I will stop pinging you.")
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
		if strings.HasPrefix(interaction.MessageComponentData().CustomID, "hil:") {
			d.approvalInteraction(interaction)
		}
		if strings.HasPrefix(interaction.MessageComponentData().CustomID, "reset:") {
			d.resetInteraction(interaction)
		}
		return
	}
	if interaction.Type == discordgo.InteractionApplicationCommandAutocomplete {
		d.agentAutocomplete(interaction)
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
	if err != nil {
		d.respond(interaction, publicFailure("checking your access", err))
		return
	}
	if role == "" {
		d.respond(interaction, accessDenied(d.cfg.BotId != ""))
		return
	}

	switch data.Name {
	case "reset":
		d.resetCommand(interaction, user)
	case "compact":
		d.compactCommand(interaction, role)
	case "usage":
		d.usageCommand(interaction, role)
	case "agent":
		d.agentCommand(interaction)
	case "mention":
		d.mentionCommand(interaction, user)
	case "user":
		d.userCommand(interaction)
	}
}
