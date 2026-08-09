package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devproje/mininaru/bot/discord/commands"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

type Config struct {
	Token   string
	BotId   string
	Agent   string
	GuildId string
}

type Discord struct {
	cfg       Config
	registry  *core.Registry
	gateway   *discordgo.Session
	approvals map[string]*discordApproval
	mu        sync.Mutex

	lifetime context.Context
	shutdown context.CancelFunc
}

const OriginDiscord = "discord"

const replyTimeout = 10 * time.Minute

func (d *Discord) turnContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(d.lifetime, replyTimeout)
}

func New(cfg Config, registry *core.Registry) (*Discord, error) {
	var bot Discord

	var err error

	if cfg.Token == "" {
		return nil, fmt.Errorf("discord token is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is required")
	}

	bot = Discord{cfg: cfg, registry: registry, approvals: make(map[string]*discordApproval)}
	bot.lifetime, bot.shutdown = context.WithCancel(context.Background())

	bot.gateway, err = discordgo.New("Bot " + cfg.Token)
	if err != nil {
		bot.shutdown()
		return nil, err
	}
	bot.gateway.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	return &bot, nil
}

func (d *Discord) role(userId string) (string, error) {
	if d.cfg.BotId == "" {
		return "", nil
	}
	return core.DiscordUserRole(d.cfg.BotId, userId)
}

func (d *Discord) configuredInstance() (*core.Instance, error) {
	if d.cfg.Agent != "" {
		return d.registry.Get(d.cfg.Agent)
	}
	return d.registry.Default()
}

func (d *Discord) instance(channelId string) (*core.Instance, error) {
	var bound *core.Session
	var target *core.Instance

	var err error

	bound, err = core.SessionByExternal(OriginDiscord, channelId)
	if err != nil {
		return nil, err
	}
	if bound != nil {
		target, err = d.registry.ByAgentId(bound.AgentId)
		if err == nil {
			return target, nil
		}
	}
	return d.configuredInstance()
}

func (d *Discord) onReady(session *discordgo.Session, ready *discordgo.Ready) {
	var err error

	err = session.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: string(discordgo.StatusOnline),
		Activities: []*discordgo.Activity{{
			Name: fmt.Sprintf("mininaru %s-%s (%s)", util.AppVersion, util.AppBranch, util.AppHash),
			Type: discordgo.ActivityTypeListening,
		}},
	})
	if err != nil {
		util.Log.Error("setting discord presence failed", "error", err)
	}
}

func (d *Discord) Start() error {
	var guildCommands []*discordgo.ApplicationCommand
	var globalCommands []*discordgo.ApplicationCommand

	var err error

	d.gateway.AddHandler(d.onReady)
	d.gateway.AddHandler(d.onMessage)
	d.gateway.AddHandler(d.onInteraction)
	if err = d.gateway.Open(); err != nil {
		return err
	}

	guildCommands, globalCommands = commands.Sets(d.cfg.GuildId)
	if len(guildCommands) > 0 {
		_, err = d.gateway.ApplicationCommandBulkOverwrite(d.gateway.State.User.ID, d.cfg.GuildId, guildCommands)
		if err != nil {
			util.Log.Error("registering discord guild commands failed", "guild", d.cfg.GuildId, "error", err)
		}
	}
	_, err = d.gateway.ApplicationCommandBulkOverwrite(d.gateway.State.User.ID, "", globalCommands)
	if err != nil {
		util.Log.Error("registering discord global commands failed", "error", err)
	}
	util.Log.Info("discord bot connected",
		"username", d.gateway.State.User.Username, "bot_id", d.cfg.BotId, "agent", d.cfg.Agent)

	return nil
}

func (d *Discord) Stop() error {
	if d.shutdown != nil {
		d.shutdown()
	}

	return d.gateway.Close()
}
