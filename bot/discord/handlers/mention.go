// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

var mentionPattern = regexp.MustCompile(`<@!?(\d+)>`)

func mentionedIds(content string) []string {
	var matches [][]string
	var match []string
	var seen map[string]bool
	var ids []string

	seen = make(map[string]bool)

	matches = mentionPattern.FindAllStringSubmatch(content, -1)
	for _, match = range matches {
		if seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		ids = append(ids, match[1])
	}

	return ids
}

func silentMentions() *discordgo.MessageAllowedMentions {
	return &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
}

func (d *Discord) allowedMentions(content string) *discordgo.MessageAllowedMentions {
	var ids []string
	var id string
	var opted map[string]bool
	var allowed []string

	var err error

	ids = mentionedIds(content)
	if len(ids) == 0 || d.cfg.BotId == "" {
		return silentMentions()
	}

	opted, err = core.DiscordMentionUsers(d.cfg.BotId)
	if err != nil {
		util.Log.Error("reading discord mention preferences failed", "bot_id", d.cfg.BotId, "error", err)

		return silentMentions()
	}

	for _, id = range ids {
		if !opted[id] {
			continue
		}
		allowed = append(allowed, id)
	}

	if len(allowed) == 0 {
		return silentMentions()
	}

	return &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}, Users: allowed}
}
