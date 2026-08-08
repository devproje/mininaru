package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/modules"
	"github.com/google/uuid"
)

type discordApproval struct {
	userId   string
	response chan bool
}

func (d *Discord) approve(ctx context.Context, channelId, userId string, def modules.Def, arguments string) (bool, error) {
	var id string
	var pending *discordApproval
	var shown string
	var approved bool

	var err error

	id = uuid.NewString()
	pending = &discordApproval{userId: userId, response: make(chan bool, 1)}
	d.mu.Lock()
	d.approvals[id] = pending
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.approvals, id)
		d.mu.Unlock()
	}()

	shown = arguments
	if len([]rune(shown)) > 1200 {
		shown = string([]rune(shown)[:1200]) + "…"
	}
	_, err = d.gateway.ChannelMessageSendComplex(channelId, &discordgo.MessageSend{
		Flags: discordgo.MessageFlagsIsComponentsV2,
		Components: v2Container(approvalAccent,
			discordgo.TextDisplay{Content: fmt.Sprintf("### 🔐 도구 승인 필요\n<@%s>만 승인할 수 있어.", userId)},
			discordgo.Separator{},
			discordgo.TextDisplay{Content: fmt.Sprintf("**🔧 `%s`**\n```json\n%s\n```", def.Name, shown)},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "승인", Emoji: &discordgo.ComponentEmoji{Name: "✅"}, Style: discordgo.SuccessButton, CustomID: "hil:yes:" + id},
				discordgo.Button{Label: "거부", Emoji: &discordgo.ComponentEmoji{Name: "✖️"}, Style: discordgo.DangerButton, CustomID: "hil:no:" + id},
			}},
		),
	})
	if err != nil {
		return false, err
	}

	select {
	case approved = <-pending.response:
		return approved, nil
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(5 * time.Minute):
		return false, fmt.Errorf("approval timed out")
	}
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
		d.respond(interaction, "this approval is not yours or has expired")
		return
	}
	components = v2Container(map[bool]int{true: statusAccent, false: approvalAccent}[approved],
		discordgo.TextDisplay{Content: fmt.Sprintf("%s **%s** · <@%s>",
			map[bool]string{true: "✅", false: "✖️"}[approved],
			map[bool]string{true: "승인됨", false: "거부됨"}[approved], pending.userId)})
	d.gateway.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsIsComponentsV2, Components: components},
	})
	select {
	case pending.response <- approved:
	default:
	}
}
