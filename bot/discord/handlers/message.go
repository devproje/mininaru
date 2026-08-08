package handlers

import (
	"context"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/bot/discord/attachments"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
)

func (d *Discord) addressed(message *discordgo.MessageCreate) (string, bool) {
	var content string
	var mention *discordgo.User

	if message.Author == nil || message.Author.Bot {
		return "", false
	}
	content = strings.TrimSpace(message.Content)
	if content == "" && len(message.Attachments) == 0 {
		return "", false
	}
	if message.GuildID == "" || d.ownedThread(message.ChannelID) {
		return content, true
	}
	for _, mention = range message.Mentions {
		if mention.ID != d.gateway.State.User.ID {
			continue
		}
		content = strings.ReplaceAll(content, "<@"+mention.ID+">", "")
		content = strings.ReplaceAll(content, "<@!"+mention.ID+">", "")
		return strings.TrimSpace(content), true
	}
	return "", false
}

func (d *Discord) ownedThread(channelId string) bool {
	var channel *discordgo.Channel

	var err error

	channel, err = d.gateway.State.Channel(channelId)
	if err != nil {
		channel, err = d.gateway.Channel(channelId)
	}
	if err != nil || channel == nil || d.gateway.State.User == nil {
		return false
	}
	if channel.Type != discordgo.ChannelTypeGuildPublicThread &&
		channel.Type != discordgo.ChannelTypeGuildPrivateThread &&
		channel.Type != discordgo.ChannelTypeGuildNewsThread {
		return false
	}
	return channel.OwnerID == d.gateway.State.User.ID
}

func (d *Discord) conversationChannel(message *discordgo.MessageCreate) string {
	var channel *discordgo.Channel
	var thread *discordgo.Channel

	var err error

	if message.GuildID == "" {
		return message.ChannelID
	}
	channel, err = d.gateway.State.Channel(message.ChannelID)
	if err == nil && (channel.Type == discordgo.ChannelTypeGuildPublicThread ||
		channel.Type == discordgo.ChannelTypeGuildPrivateThread || channel.Type == discordgo.ChannelTypeGuildNewsThread) {
		return message.ChannelID
	}
	thread, err = d.gateway.MessageThreadStart(message.ChannelID, message.ID, "mininaru", 60)
	if err != nil {
		return message.ChannelID
	}
	return thread.ID
}

func (d *Discord) answerFor(ctx context.Context, channelId, sourceChannelId, sourceMessageId, userId, role, content string,
	sourceAttachments []*discordgo.MessageAttachment) {
	var target *core.Instance
	var session *core.Session
	var indicator *typing
	var status *executionStatus
	var parts []openai.ChatCompletionContentPartUnionParam
	var onReasoning func(string)
	var reasoningOnce sync.Once
	var onTool core.ToolEventFunc
	var defs []modules.Def
	var message *core.Message

	var err error

	target, err = d.instance(channelId)
	if err != nil {
		sendReply(d.gateway, channelId, publicFailure("agent lookup", err))
		return
	}
	session, err = target.Bind(OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		sendReply(d.gateway, channelId, publicFailure("session setup", err))
		return
	}
	indicator = startTyping(d.gateway, channelId)
	status = newExecutionStatus(d.gateway, channelId, sourceChannelId, sourceMessageId)
	if len(sourceAttachments) > 0 {
		parts, err = attachments.Build(ctx, content, sourceAttachments)
		if err != nil {
			indicator.stop()
			status.show("❌", "❌ **첨부 처리 실패**")
			sendReply(d.gateway, channelId, publicFailure("attachment processing", err))
			return
		}
	}
	onReasoning = func(text string) {
		reasoningOnce.Do(func() { status.show("⚡", "⚡ **추론 중**") })
	}
	onTool = func(event core.ToolEvent) {
		if event.Phase == core.ToolEventStarted {
			status.show("🔧", "🔧 **툴 실행 중** · `"+event.Name+"`")
		}
	}
	if role == core.DiscordRoleAdmin {
		defs = modules.DefaultTools()
		message, err = target.ChatInput(ctx, session, content, parts, defs, onReasoning, onTool,
			func(ctx context.Context, def modules.Def, arguments string) (bool, error) {
				return d.approve(ctx, channelId, userId, def, arguments)
			})
	} else {
		message, err = target.ChatInput(ctx, session, content, parts, target.Tools, onReasoning, onTool, nil)
	}
	indicator.stop()
	if err != nil {
		status.show("❌", "❌ **실행 실패**")
		sendReply(d.gateway, channelId, publicFailure("request", err))
		return
	}
	status.show("✅", "✅ **완료**")
	sendReply(d.gateway, channelId, message.Content)
}

func (d *Discord) onMessage(gateway *discordgo.Session, message *discordgo.MessageCreate) {
	var content string
	var addressed bool
	var role string
	var channelId string

	var err error

	content, addressed = d.addressed(message)
	if !addressed {
		return
	}
	role, err = d.role(message.Author.ID)
	if err != nil || role == "" {
		sendReply(gateway, message.ChannelID, "you are not paired with this bot")
		return
	}
	channelId = d.conversationChannel(message)

	go func() {
		var ctx context.Context
		var cancel context.CancelFunc

		ctx, cancel = d.turnContext()
		defer cancel()

		d.answerFor(ctx, channelId, message.ChannelID, message.ID, message.Author.ID, role, content, message.Attachments)
	}()
}
