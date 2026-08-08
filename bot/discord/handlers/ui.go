package handlers

import (
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

const statusAccent = 0xF0B232
const approvalAccent = 0xED4245

func v2Container(accent int, components ...discordgo.MessageComponent) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{discordgo.Container{AccentColor: &accent, Components: components}}
}

func contextResultComponents(title, content string, extra ...discordgo.MessageComponent) []discordgo.MessageComponent {
	var runes []rune
	var body []discordgo.MessageComponent

	runes = []rune(content)
	if len(runes) > 3800 {
		content = string(runes[:3800]) + "…"
	}
	body = []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: "### " + title},
		discordgo.Separator{},
		discordgo.TextDisplay{Content: content},
	}
	body = append(body, extra...)
	return v2Container(statusAccent, body...)
}

func newExecutionStatus(gateway *discordgo.Session, channelId, sourceChannelId, sourceMessageId string) *executionStatus {
	var status *executionStatus
	var message *discordgo.Message

	var err error

	status = &executionStatus{
		gateway: gateway, channelId: channelId, sourceChannelId: sourceChannelId, sourceMessageId: sourceMessageId,
		current: "💡 **스킬 선택 중**", currentReaction: "💡",
	}
	message, err = gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
		Flags:      discordgo.MessageFlagsIsComponentsV2,
		Components: v2Container(statusAccent, discordgo.TextDisplay{Content: status.current}),
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
