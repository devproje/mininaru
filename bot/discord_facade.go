// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package bot

import (
	"github.com/devproje/mininaru/bot/discord"
	"github.com/devproje/mininaru/core"
)

type Discord = discord.Discord
type DiscordConfig = discord.Config

func NewDiscord(cfg DiscordConfig, registry *core.Registry) (*Discord, error) {
	return discord.New(cfg, registry)
}
