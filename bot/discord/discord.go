package discord

import (
	"github.com/devproje/mininaru/bot/discord/handlers"
	"github.com/devproje/mininaru/core"
)

type Config = handlers.Config
type Discord = handlers.Discord

func New(cfg Config, registry *core.Registry) (*Discord, error) {
	return handlers.New(cfg, registry)
}
