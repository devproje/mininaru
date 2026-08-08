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
