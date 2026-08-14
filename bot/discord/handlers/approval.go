// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/modules"
	"github.com/google/uuid"
)

type approvalPresentation struct {
	title   string
	target  string
	impact  string
	details string
}

type discordApproval struct {
	userId    string
	channelId string
	messageId string
	response  chan bool
}

const approvalTimeout = 5 * time.Minute
const approvalDetailsLimit = 1200
const approvalTitleLimit = 160

func (d *Discord) approve(ctx context.Context, channelId, userId string, def modules.Def, arguments string) (bool, error) {
	var id string
	var view approvalPresentation
	var pending *discordApproval
	var sent *discordgo.Message
	var approved bool
	var timer *time.Timer

	var err error

	id = uuid.NewString()
	view = approvalView(def, arguments)
	pending = &discordApproval{userId: userId, channelId: channelId, response: make(chan bool, 1)}
	d.mu.Lock()
	d.approvals[id] = pending
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.approvals, id)
		d.mu.Unlock()
	}()

	sent, err = d.gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		AllowedMentions: d.allowedMentions("<@" + userId + ">"),
		Components:      approvalRequestComponents(id, userId, view),
	})
	if err != nil {
		return false, err
	}
	pending.messageId = sent.ID
	timer = time.NewTimer(approvalTimeout)
	defer timer.Stop()

	select {
	case approved = <-pending.response:
		return approved, nil
	case <-ctx.Done():
		d.closeApproval(pending, "canceled")
		return false, ctx.Err()
	case <-timer.C:
		d.closeApproval(pending, "expired")
		return false, fmt.Errorf("approval timed out")
	}
}

func approvalView(def modules.Def, arguments string) approvalPresentation {
	var payload map[string]any
	var view approvalPresentation
	var action string
	var scope string

	payload, view.details = approvalArguments(arguments)
	view.title = strings.Join(strings.Fields(def.Description), " ")
	view.impact = "This tool can access resources outside the conversation."

	switch def.Name {
	case "file_read":
		view.title = "Read a local file"
		view.target = approvalString(payload, "path")
		view.impact = "The file contents will be shared with the agent."
	case "file_write":
		view.title = "Change a local file"
		view.target = approvalString(payload, "path")
		view.impact = "This can create, replace, or append to a file on disk."
	case "bash_exec":
		view.title = "Run a shell command"
		view.target = approvalString(payload, "command")
		view.impact = "The command can read or change files and start other processes."
	case modules.MemoryToolName:
		action = approvalString(payload, "action")
		view.title = "Manage persistent memory"
		view.target = action
		view.impact = "This can change information included in future conversations."
	case modules.SkillCreateToolName:
		view.title = "Create or replace a skill"
		view.target = approvalString(payload, "name")
		scope = approvalString(payload, "scope")
		if scope != "" {
			view.target += " · " + scope
		}
		view.impact = "This changes persistent instructions available in future conversations."
	}
	if view.title == "" {
		view.title = def.Name
	}
	if len([]rune(view.title)) > approvalTitleLimit {
		view.title = string([]rune(view.title)[:approvalTitleLimit]) + "…"
	}
	if len([]rune(view.target)) > 240 {
		view.target = string([]rune(view.target)[:240]) + "…"
	}
	return view
}

func approvalArguments(arguments string) (map[string]any, string) {
	var payload map[string]any
	var formatted bytes.Buffer
	var shown string

	var err error

	err = json.Unmarshal([]byte(arguments), &payload)
	if err == nil {
		err = json.Indent(&formatted, []byte(arguments), "", "  ")
	}
	if err == nil {
		shown = formatted.String()
	} else {
		shown = arguments
	}
	shown = strings.ReplaceAll(shown, "```", "`\u200b``")
	if len([]rune(shown)) > approvalDetailsLimit {
		shown = string([]rune(shown)[:approvalDetailsLimit]) + "…"
	}
	return payload, shown
}

func approvalString(payload map[string]any, key string) string {
	var value any
	var found bool
	var text string

	value, found = payload[key]
	if !found {
		return ""
	}
	text, found = value.(string)
	if !found {
		return ""
	}
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r", "")
	return strings.ReplaceAll(text, "\n", " ↵ ")
}

func approvalRequestComponents(id, userId string, view approvalPresentation) []discordgo.MessageComponent {
	var summary string
	var body []discordgo.MessageComponent

	summary = "### 🔐 " + view.title
	if view.target != "" {
		summary += "\n**Target:** `" + strings.ReplaceAll(view.target, "`", "ˋ") + "`"
	}
	summary += "\n**Impact:** " + view.impact
	body = []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: summary},
		discordgo.Separator{},
		discordgo.TextDisplay{Content: fmt.Sprintf("Only <@%s> can decide. This request expires in 5 minutes.", userId)},
	}
	if view.details != "" {
		body = append(body, discordgo.TextDisplay{Content: "**Details**\n```json\n" + view.details + "\n```"})
	}
	body = append(body, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{Label: "Approve once", Emoji: &discordgo.ComponentEmoji{Name: "✅"}, Style: discordgo.SuccessButton, CustomID: "hil:yes:" + id},
		discordgo.Button{Label: "Deny", Emoji: &discordgo.ComponentEmoji{Name: "✖️"}, Style: discordgo.DangerButton, CustomID: "hil:no:" + id},
	}})
	return v2Container(approvalAccent, body...)
}

func approvalResultComponents(approved bool, userId string) []discordgo.MessageComponent {
	return v2Container(map[bool]int{true: statusAccent, false: approvalAccent}[approved],
		discordgo.TextDisplay{Content: fmt.Sprintf("%s **%s** · <@%s>",
			map[bool]string{true: "✅", false: "✖️"}[approved],
			map[bool]string{true: "Approved once", false: "Denied"}[approved], userId)})
}

func approvalClosedComponents(state string) []discordgo.MessageComponent {
	return v2Container(approvalAccent,
		discordgo.TextDisplay{Content: "⌛ **Approval " + state + "**\nNo tool action was taken."})
}

func (d *Discord) closeApproval(pending *discordApproval, state string) {
	var components []discordgo.MessageComponent

	components = approvalClosedComponents(state)
	d.gateway.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID: pending.messageId, Channel: pending.channelId, Flags: discordgo.MessageFlagsIsComponentsV2, Components: &components,
		AllowedMentions: silentMentions(),
	})
}

func (d *Discord) approvalInteraction(interaction *discordgo.InteractionCreate) {
	var customId string
	var approved bool
	var id string
	var pending *discordApproval
	var found bool
	var actor *discordgo.User
	var components []discordgo.MessageComponent

	customId = interaction.MessageComponentData().CustomID
	approved = strings.HasPrefix(customId, "hil:yes:")
	id = strings.TrimPrefix(strings.TrimPrefix(customId, "hil:yes:"), "hil:no:")
	d.mu.Lock()
	pending, found = d.approvals[id]
	d.mu.Unlock()
	actor = interactionUser(interaction)
	if !found || actor == nil || actor.ID != pending.userId {
		d.respond(interaction, "That approval is not yours, or it already expired.")
		return
	}
	components = approvalResultComponents(approved, pending.userId)
	d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsIsComponentsV2, Components: components,
			AllowedMentions: d.allowedMentions("<@" + pending.userId + ">"),
		},
	})
	select {
	case pending.response <- approved:
	default:
	}
}
