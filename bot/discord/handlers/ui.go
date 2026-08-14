// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type statusStep struct {
	icon string
	text string
}

type executionStatus struct {
	gateway         *discordgo.Session
	channelId       string
	messageId       string
	sourceChannelId string
	sourceMessageId string
	note            string
	stateIcon       string
	stateText       string
	steps           []statusStep
	footer          string
	rendered        string
	started         time.Time
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

const maxStatusSteps = 20

const statusThinkingIcon = "⏳"
const statusThinkingText = "Thinking"

const threadStartedNote = "This thread keeps its own context. Continue here without mentioning me, or use `/reset` for a fresh one."

const threadFallbackNote = "I could not start a thread, so this conversation shares the channel's context. Check that the bot can create public threads here."

func v2Container(accent int, components ...discordgo.MessageComponent) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{discordgo.Container{AccentColor: &accent, Components: components}}
}

func userAppComponents(view userAppPresentation, state, content string) []discordgo.MessageComponent {
	var runes []rune
	var icon string
	var accent int
	var body []discordgo.MessageComponent

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

func newExecutionStatus(gateway *discordgo.Session, channelId, sourceChannelId, sourceMessageId, note string) *executionStatus {
	var status *executionStatus
	var components []discordgo.MessageComponent
	var message *discordgo.Message

	var err error

	status = &executionStatus{
		gateway: gateway, channelId: channelId, sourceChannelId: sourceChannelId, sourceMessageId: sourceMessageId,
		note: note, stateIcon: statusThinkingIcon, stateText: statusThinkingText, started: time.Now(),
	}
	status.rendered = status.render()

	components = v2Container(statusAccent, discordgo.TextDisplay{Content: status.rendered})
	message, err = gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		Components:      components,
		AllowedMentions: silentMentions(),
	})
	if err == nil {
		status.messageId = message.ID
	}

	status.react(statusThinkingIcon)

	return status
}

func (s *executionStatus) render() string {
	var lines []string
	var steps []statusStep
	var step statusStep
	var hidden int
	var text string
	var runes []rune

	lines = append(lines, "### "+s.stateIcon+" "+s.stateText)
	if s.note != "" {
		lines = append(lines, "-# "+s.note)
	}

	steps = s.steps
	if len(steps) > maxStatusSteps {
		hidden = len(steps) - maxStatusSteps
		steps = steps[len(steps)-maxStatusSteps:]
	}

	if hidden > 0 {
		lines = append(lines, "", "-# "+strconv.Itoa(hidden)+" earlier steps hidden")
	} else if len(steps) > 0 {
		lines = append(lines, "")
	}

	for _, step = range steps {
		lines = append(lines, step.icon+" "+step.text)
	}

	if s.footer != "" {
		lines = append(lines, "-# "+s.footer)
	}

	text = strings.Join(lines, "\n")

	runes = []rune(text)
	if len(runes) > messageLimit {
		return string(runes[:messageLimit])
	}

	return text
}

func (s *executionStatus) publish() {
	var text string
	var accent int
	var components []discordgo.MessageComponent

	if s.messageId == "" {
		return
	}

	text = s.render()
	if text == s.rendered {
		return
	}
	s.rendered = text

	accent = statusAccent
	if s.stateIcon == "❌" {
		accent = approvalAccent
	}

	components = v2Container(accent, discordgo.TextDisplay{Content: text})
	s.gateway.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID: s.messageId, Channel: s.channelId, Flags: discordgo.MessageFlagsIsComponentsV2,
		Components: &components, AllowedMentions: silentMentions(),
	})
}

func (s *executionStatus) react(reaction string) {
	if s.sourceMessageId == "" || reaction == "" || reaction == s.currentReaction {
		return
	}

	if s.currentReaction != "" {
		s.gateway.MessageReactionRemove(s.sourceChannelId, s.sourceMessageId, s.currentReaction, "@me")
	}

	s.gateway.MessageReactionAdd(s.sourceChannelId, s.sourceMessageId, reaction)
	s.currentReaction = reaction
}

func (s *executionStatus) progress(icon, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stateIcon = icon
	s.stateText = text
	s.publish()
	s.react(icon)
}

func (s *executionStatus) log(icon, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.steps = append(s.steps, statusStep{icon: icon, text: text})
	s.stateIcon = statusThinkingIcon
	s.stateText = statusThinkingText
	s.publish()
}

func statusFooter(elapsed time.Duration, steps int) string {
	var parts []string

	parts = append(parts, elapsed.Truncate(100*time.Millisecond).String())

	if steps == 1 {
		parts = append(parts, "1 tool")
	}
	if steps > 1 {
		parts = append(parts, strconv.Itoa(steps)+" tools")
	}

	return strings.Join(parts, " · ")
}

func (s *executionStatus) finish(icon, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stateIcon = icon
	s.stateText = text
	s.footer = statusFooter(time.Since(s.started), len(s.steps))
	s.publish()
	s.react(icon)
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
