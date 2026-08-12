// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type executionStatus struct {
	gateway         *discordgo.Session
	channelId       string
	messageId       string
	sourceChannelId string
	sourceMessageId string
	current         string
	currentReaction string
	mu              sync.Mutex
}

type userAppPresentation struct {
	title string
	note  string
}

const statusAccent = 0xF0B232
const approvalAccent = 0xED4245
const userAppContentLimit = 3400
const userAppWorking = "working"
const userAppDone = "done"
const userAppFailed = "failed"

func v2Container(accent int, components ...discordgo.MessageComponent) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{discordgo.Container{AccentColor: &accent, Components: components}}
}

func userAppComponents(view userAppPresentation, state, content string) []discordgo.MessageComponent {
	var runes []rune
	var body []discordgo.MessageComponent
	var icon string
	var accent int

	if strings.TrimSpace(content) == "" {
		content = emptyReply
	}
	runes = []rune(content)
	if len(runes) > userAppContentLimit {
		content = string(runes[:userAppContentLimit]) + "…\n\n_Result shortened to fit Discord._"
	}
	icon = "💡"
	accent = statusAccent
	if state == userAppDone {
		icon = "✅"
	}
	if state == userAppFailed {
		icon = "❌"
		accent = approvalAccent
	}
	body = []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: "### " + icon + " " + view.title},
		discordgo.Separator{},
		discordgo.TextDisplay{Content: content},
		discordgo.TextDisplay{Content: "-# " + view.note},
	}
	return v2Container(accent, body...)
}

func (d *Discord) sendThreadWelcome(channelId, agentName string) {
	d.gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		Components:      threadWelcomeComponents(agentName),
		AllowedMentions: silentMentions(),
	})
}

func threadWelcomeComponents(agentName string) []discordgo.MessageComponent {
	var shownName string

	shownName = strings.ReplaceAll(agentName, "`", "ˋ")
	return v2Container(statusAccent,
		discordgo.TextDisplay{Content: "### 👋 Conversation started\nThis thread keeps its own context with `" + shownName + "`. You can continue here without mentioning me."},
		discordgo.TextDisplay{Content: "-# Use `/reset` for a fresh context or `/agent` to view and switch agents."},
	)
}

func (d *Discord) sendThreadFallback(channelId string) {
	d.gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		Components:      threadFallbackComponents(),
		AllowedMentions: silentMentions(),
	})
}

func threadFallbackComponents() []discordgo.MessageComponent {
	return v2Container(approvalAccent,
		discordgo.TextDisplay{Content: "### ⚠️ Could not start a conversation thread\nI will answer in this channel instead, so later messages here may share the same conversation context."},
		discordgo.TextDisplay{Content: "-# Check that the bot can create public threads in this channel."},
	)
}

func newExecutionStatus(gateway *discordgo.Session, channelId, sourceChannelId, sourceMessageId string) *executionStatus {
	var status *executionStatus
	var message *discordgo.Message

	var err error

	status = &executionStatus{
		gateway: gateway, channelId: channelId, sourceChannelId: sourceChannelId, sourceMessageId: sourceMessageId,
		current: "💡 **Picking a skill**", currentReaction: "💡",
	}
	message, err = gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		Components:      v2Container(statusAccent, discordgo.TextDisplay{Content: status.current}),
		AllowedMentions: silentMentions(),
	})
	if err == nil {
		status.messageId = message.ID
	}
	if sourceMessageId != "" {
		gateway.MessageReactionAdd(sourceChannelId, sourceMessageId, status.currentReaction)
	}
	return status
}

func (s *executionStatus) show(reaction, text string) {
	var components []discordgo.MessageComponent

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == text {
		return
	}
	s.current = text
	if s.messageId != "" {
		components = v2Container(statusAccent, discordgo.TextDisplay{Content: text})
		s.gateway.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID: s.messageId, Channel: s.channelId, Flags: discordgo.MessageFlagsIsComponentsV2, Components: &components,
			AllowedMentions: silentMentions(),
		})
	}
	if s.sourceMessageId != "" && reaction != s.currentReaction {
		if s.currentReaction != "" {
			s.gateway.MessageReactionRemove(s.sourceChannelId, s.sourceMessageId, s.currentReaction, "@me")
		}
		s.gateway.MessageReactionAdd(s.sourceChannelId, s.sourceMessageId, reaction)
		s.currentReaction = reaction
	}
}

func (s *executionStatus) finish(reaction, text string, allowed *discordgo.MessageAllowedMentions) []string {
	var chunks []string
	var components []discordgo.MessageComponent
	var accent int

	var err error

	if text == "" {
		text = emptyReply
	}
	chunks = splitReply(text, messageLimit)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = chunks[0]
	if s.messageId == "" {
		return chunks
	}
	accent = statusAccent
	if reaction == "❌" {
		accent = approvalAccent
	}
	components = v2Container(accent, discordgo.TextDisplay{Content: chunks[0]})
	_, err = s.gateway.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID: s.messageId, Channel: s.channelId, Flags: discordgo.MessageFlagsIsComponentsV2, Components: &components,
		AllowedMentions: allowed,
	})
	if err != nil {
		return chunks
	}
	if s.sourceMessageId != "" && reaction != s.currentReaction {
		if s.currentReaction != "" {
			s.gateway.MessageReactionRemove(s.sourceChannelId, s.sourceMessageId, s.currentReaction, "@me")
		}
		s.gateway.MessageReactionAdd(s.sourceChannelId, s.sourceMessageId, reaction)
		s.currentReaction = reaction
	}
	return chunks[1:]
}

func interactionUser(interaction *discordgo.InteractionCreate) *discordgo.User {
	if interaction == nil {
		return nil
	}
	if interaction.User != nil {
		return interaction.User
	}
	if interaction.Member != nil {
		return interaction.Member.User
	}
	return nil
}
