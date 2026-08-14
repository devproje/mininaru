// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"context"
	"strings"
	"sync"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/bot/discord/attachments"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	"github.com/openai/openai-go"
)

type conversationTarget struct {
	channelId string
	created   bool
	fallback  bool
}

const threadNameLimit = 100

const toolReasonLimit = 120

const seenMessageLimit = 2048

func (t conversationTarget) note() string {
	if t.fallback {
		return threadFallbackNote
	}
	if t.created {
		return threadStartedNote
	}

	return ""
}

func replyTarget(channelId, sourceChannelId, sourceMessageId string) string {
	if sourceChannelId != channelId {
		return ""
	}

	return sourceMessageId
}

func toolFailureReason(text string) string {
	var reason string

	reason = strings.Join(strings.Fields(text), " ")
	if reason == "" {
		return "failed"
	}

	if len([]rune(reason)) > toolReasonLimit {
		reason = string([]rune(reason)[:toolReasonLimit]) + "…"
	}

	return reason
}

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

func (d *Discord) conversationChannel(message *discordgo.MessageCreate, content string) conversationTarget {
	var target conversationTarget
	var channel *discordgo.Channel
	var thread *discordgo.Channel

	var err error

	target.channelId = message.ChannelID
	if message.GuildID == "" {
		return target
	}
	channel, err = d.gateway.State.Channel(message.ChannelID)
	if err != nil {
		channel, err = d.gateway.Channel(message.ChannelID)
	}
	if err == nil && (channel.Type == discordgo.ChannelTypeGuildPublicThread ||
		channel.Type == discordgo.ChannelTypeGuildPrivateThread || channel.Type == discordgo.ChannelTypeGuildNewsThread) {
		return target
	}
	thread, err = d.gateway.MessageThreadStart(message.ChannelID, message.ID, threadName(content, len(message.Attachments) > 0), 60)
	if err != nil {
		target.fallback = true
		util.Log.Warn("creating discord conversation thread failed",
			"channel", message.ChannelID, "message", message.ID, "error", err)
		return target
	}
	target.channelId = thread.ID
	target.created = true
	return target
}

func threadName(content string, attached bool) string {
	var summary string
	var name string
	var runes []rune

	summary = strings.Join(strings.Fields(content), " ")
	summary = strings.Map(func(value rune) rune {
		if unicode.IsControl(value) {
			return -1
		}
		return value
	}, summary)
	summary = strings.Trim(summary, "#*_`>~ ")
	if summary == "" && attached {
		summary = "attachment"
	}
	if summary == "" {
		summary = "conversation"
	}
	name = "mininaru · " + summary
	runes = []rune(name)
	if len(runes) > threadNameLimit {
		name = string(runes[:threadNameLimit-1]) + "…"
	}
	return name
}

func (d *Discord) rememberMessage(messageId string) bool {
	var found bool
	var oldest string

	if messageId == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen == nil {
		d.seen = make(map[string]struct{})
	}
	_, found = d.seen[messageId]
	if found {
		return false
	}
	if len(d.seenOrder) == seenMessageLimit {
		oldest = d.seenOrder[0]
		delete(d.seen, oldest)
		d.seenOrder = d.seenOrder[1:]
	}
	d.seen[messageId] = struct{}{}
	d.seenOrder = append(d.seenOrder, messageId)
	return true
}

func (d *Discord) queueTurn(channelId string, run func()) {
	var current chan struct{}
	var previous chan struct{}

	current = make(chan struct{})
	d.mu.Lock()
	if d.turns == nil {
		d.turns = make(map[string]chan struct{})
	}
	previous = d.turns[channelId]
	d.turns[channelId] = current
	d.mu.Unlock()

	go func() {
		defer func() {
			close(current)
			d.mu.Lock()
			if d.turns[channelId] == current {
				delete(d.turns, channelId)
			}
			d.mu.Unlock()
		}()
		if previous != nil {
			if d.lifetime == nil {
				<-previous
			} else {
				select {
				case <-previous:
				case <-d.lifetime.Done():
					return
				}
			}
		}
		if d.lifetime != nil && d.lifetime.Err() != nil {
			return
		}
		run()
	}()
}

func (d *Discord) answerFor(ctx context.Context, channelId, sourceChannelId, sourceMessageId, userId, role, content string,
	sourceAttachments []*discordgo.MessageAttachment, note string) {
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
	var replyTo string

	var err error

	replyTo = replyTarget(channelId, sourceChannelId, sourceMessageId)

	target, err = d.instance(channelId)
	if err != nil {
		d.sendReply(channelId, conversationFailure("looking up the agent", err))
		return
	}
	session, err = target.Bind(OriginDiscord, channelId, "discord "+channelId)
	if err != nil {
		d.sendReply(channelId, conversationFailure("setting up the conversation", err))
		return
	}
	indicator = startTyping(d.gateway, channelId)
	status = newExecutionStatus(d.gateway, channelId, sourceChannelId, sourceMessageId, note)
	if len(sourceAttachments) > 0 {
		parts, err = attachments.Build(ctx, content, sourceAttachments)
		if err != nil {
			indicator.stop()
			status.finish("❌", "Failed")
			d.sendReplyTo(channelId, replyTo, conversationFailure("reading the attachment", err))
			return
		}
	}
	onReasoning = func(text string) {
		reasoningOnce.Do(func() { status.progress("⚡", "Reasoning") })
	}
	onTool = func(event core.ToolEvent) {
		var label string

		label = core.ToolLabel(event.Name, event.Arguments)

		if event.Phase == core.ToolEventStarted {
			status.progress("🔧", "Running `"+label+"`")
			return
		}

		if event.Status == core.MessageCompleted {
			status.log("✓", "`"+label+"`")
			return
		}

		status.log("✗", "`"+label+"` — "+toolFailureReason(event.Error))
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
		status.finish("❌", "Failed")
		d.sendReplyTo(channelId, replyTo, conversationFailure("answering", err))
		return
	}
	status.finish("✅", "Answered")
	d.sendReplyTo(channelId, replyTo, message.Content)
}

func (d *Discord) onMessage(gateway *discordgo.Session, message *discordgo.MessageCreate) {
	var content string
	var addressed bool
	var role string
	var target conversationTarget

	var err error

	content, addressed = d.addressed(message)
	if !addressed {
		return
	}
	role, err = d.role(message.Author.ID)
	if err != nil {
		d.sendReply(message.ChannelID, publicFailure("checking your access", err))
		return
	}
	if role == "" {
		util.Log.Debug("ignoring a mention from an unpaired user",
			"user", message.Author.ID, "channel", message.ChannelID)
		d.sendReply(message.ChannelID, accessDenied(d.cfg.BotId != ""))
		return
	}
	if !d.rememberMessage(message.ID) {
		util.Log.Debug("ignoring duplicate discord message", "message", message.ID, "channel", message.ChannelID)
		return
	}
	target = d.conversationChannel(message, content)
	content = withIdentity(message.Author.ID, role, content)
	d.queueTurn(target.channelId, func() {
		var ctx context.Context
		var cancel context.CancelFunc

		ctx, cancel = d.turnContext()
		defer cancel()

		d.answerFor(ctx, target.channelId, message.ChannelID, message.ID, message.Author.ID, role, content, message.Attachments, target.note())
	})
}
