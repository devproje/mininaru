package handlers

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/bot/discord/attachments"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
)

func userInstallOwner(interaction *discordgo.InteractionCreate, userId string) bool {
	var owner string
	var found bool

	if interaction == nil || interaction.Interaction == nil {
		return false
	}
	owner, found = interaction.AuthorizingIntegrationOwners[discordgo.ApplicationIntegrationUserInstall]
	return found && owner == userId
}

func contextMessage(interaction *discordgo.InteractionCreate) *discordgo.Message {
	var data discordgo.ApplicationCommandInteractionData

	data = interaction.ApplicationCommandData()
	if data.Resolved == nil {
		return nil
	}
	return data.Resolved.Messages[data.TargetID]
}

func contextPrompt(name string) (string, []modules.Def) {
	var prompt string
	var defs []modules.Def

	switch name {
	case "Message Analyzer":
		prompt = `Analyze the selected message as text, without diagnosing the author's personality or mental state.
Identify its likely intent, tone, explicit request, ambiguity, hidden assumptions, and anything the reader should verify.
Clearly label uncertain inferences. Answer in the message's language using concise Markdown.`
	case "Content Search":
		prompt = `Search the public web for the selected message's central topic and any verifiable claims.
Return a concise synthesis with direct source links. Separate confirmed facts, conflicting information, and claims that could not be verified.
Do not invent citations. Answer in the message's language.`
		defs = []modules.Def{modules.WebSearch()}
	}
	return prompt, defs
}

func (d *Discord) contextCommand(interaction *discordgo.InteractionCreate, user *discordgo.User) {
	var selected *discordgo.Message
	var target *core.Instance
	var flags discordgo.MessageFlags
	var title string

	var err error

	if !userInstallOwner(interaction, user.ID) {
		d.respond(interaction, "this command requires the app to be installed to your user account")
		return
	}
	selected = contextMessage(interaction)
	if selected == nil || strings.TrimSpace(selected.Content) == "" {
		d.respond(interaction, "the selected message has no text content")
		return
	}
	target, err = d.configuredInstance()
	if err != nil {
		d.respond(interaction, publicFailure("agent lookup", err))
		return
	}
	flags = discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral
	title = interaction.ApplicationCommandData().Name
	err = d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: flags, Components: contextResultComponents(title, "💡 **처리 중**")},
	})
	if err == nil {
		go func() {
			var ctx context.Context
			var cancel context.CancelFunc

			ctx, cancel = d.turnContext()
			defer cancel()

			d.runContextCommand(ctx, interaction, target, selected.Content, selected.Attachments)
		}()
	}
}

func (d *Discord) runContextCommand(ctx context.Context, interaction *discordgo.InteractionCreate, target *core.Instance, content string,
	files []*discordgo.MessageAttachment) {
	var title string
	var prompt string
	var defs []modules.Def
	var parts []openai.ChatCompletionContentPartUnionParam
	var components []discordgo.MessageComponent
	var messages []openai.ChatCompletionMessageParamUnion
	var result *core.Completion

	var err error

	title = interaction.ApplicationCommandData().Name
	prompt, defs = contextPrompt(title)
	if len(files) > 0 {
		parts, err = attachments.Build(ctx, content, files)
		if err != nil {
			publicFailure("attachment processing", err)
			components = contextResultComponents(title, "❌ 첨부 처리 실패")
			d.gateway.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{Components: &components})
			return
		}
		messages = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(prompt), openai.UserMessage(parts)}
	} else {
		messages = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(prompt), openai.UserMessage(content)}
	}
	result, err = core.Complete(ctx, target.Agent, messages, defs, "", nil, nil)
	if err != nil {
		publicFailure("context command", err)
		components = contextResultComponents(title, "❌ 처리 실패")
	} else {
		components = contextResultComponents(title, result.Content)
	}
	d.gateway.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{Components: &components})
}

func (d *Discord) chatCommand(interaction *discordgo.InteractionCreate, user *discordgo.User) {
	var ephemeral bool
	var options []*discordgo.ApplicationCommandInteractionDataOption
	var option *discordgo.ApplicationCommandInteractionDataOption
	var content string
	var attachmentId string
	var valid bool
	var attachment *discordgo.MessageAttachment
	var target *core.Instance
	var flags discordgo.MessageFlags
	var files []*discordgo.MessageAttachment

	var err error

	if !userInstallOwner(interaction, user.ID) {
		d.respond(interaction, "this command requires the app to be installed to your user account")
		return
	}
	ephemeral = true
	options = interaction.ApplicationCommandData().Options
	for _, option = range options {
		switch option.Name {
		case "content":
			content = strings.TrimSpace(option.StringValue())
		case "ephemeral":
			ephemeral = option.BoolValue()
		case "attachment":
			if interaction.ApplicationCommandData().Resolved != nil {
				attachmentId, valid = option.Value.(string)
				if valid {
					attachment = interaction.ApplicationCommandData().Resolved.Attachments[attachmentId]
				}
			}
		}
	}
	if content == "" {
		d.respond(interaction, "content is required")
		return
	}
	target, err = d.configuredInstance()
	if err != nil {
		d.respond(interaction, publicFailure("agent lookup", err))
		return
	}
	flags = discordgo.MessageFlagsIsComponentsV2
	if ephemeral {
		flags |= discordgo.MessageFlagsEphemeral
	}
	err = d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: flags, Components: contextResultComponents("Stateless Chat", "💡 **처리 중**")},
	})
	if err == nil {
		if attachment != nil {
			files = []*discordgo.MessageAttachment{attachment}
		}
		go func() {
			var ctx context.Context
			var cancel context.CancelFunc

			ctx, cancel = d.turnContext()
			defer cancel()

			d.runStatelessChat(ctx, interaction, target, content, files)
		}()
	}
}

func (d *Discord) runStatelessChat(ctx context.Context, interaction *discordgo.InteractionCreate, target *core.Instance, content string,
	files []*discordgo.MessageAttachment) {
	var parts []openai.ChatCompletionContentPartUnionParam
	var components []discordgo.MessageComponent
	var messages []openai.ChatCompletionMessageParamUnion
	var result *core.Completion

	var err error

	if len(files) > 0 {
		parts, err = attachments.Build(ctx, content, files)
		if err != nil {
			publicFailure("attachment processing", err)
			components = contextResultComponents("Stateless Chat", "❌ 첨부 처리 실패")
			d.gateway.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{Components: &components})
			return
		}
		messages = []openai.ChatCompletionMessageParamUnion{openai.UserMessage(parts)}
	} else {
		messages = []openai.ChatCompletionMessageParamUnion{openai.UserMessage(content)}
	}
	result, err = core.Complete(ctx, target.Agent, messages, nil, "", nil, nil)
	if err != nil {
		publicFailure("stateless chat", err)
		components = contextResultComponents("Stateless Chat", "❌ 처리 실패")
	} else {
		components = contextResultComponents("Stateless Chat", result.Content)
	}
	d.gateway.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{Components: &components})
}
