package core

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type Bot struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Token   string `json:"token"`
	Agent   string `json:"agent"`
	GuildId string `json:"guild_id"`
	Enabled bool   `json:"enabled"`
}

type BotConfig struct {
	Bots []*Bot `json:"bots"`
}

const BOT_PATH = "bot.json"

const BotDiscord = "discord"

var Bots []*Bot

var emptyBotObj BotConfig = BotConfig{Bots: []*Bot{}}

func BotKinds() []string {
	return []string{BotDiscord}
}

func BotKindValid(kind string) bool {
	var cur string

	for _, cur = range BotKinds() {
		if cur != kind {
			continue
		}

		return true
	}

	return false
}

func BotInit() error {
	var path string
	var buf []byte
	var cfg BotConfig

	var err error

	path = util.Path(BOT_PATH)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		buf, _ = json.MarshalIndent(emptyBotObj, "", "    ")

		err = util.WriteFileAtomic(path, buf, 0600)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal(buf, &cfg)
	if err != nil {
		return err
	}

	Bots = cfg.Bots

	return nil
}

func BotSave() error {
	var cfg BotConfig
	var path string
	var buf []byte

	var err error

	cfg = BotConfig{Bots: Bots}
	if cfg.Bots == nil {
		cfg.Bots = []*Bot{}
	}

	path = util.Path(BOT_PATH)
	buf, err = json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(path, buf, 0600)
}

func BotFind(ref string) (*Bot, error) {
	var cur *Bot

	var err error

	for _, cur = range Bots {
		if cur.Id != ref {
			continue
		}

		return cur, nil
	}

	for _, cur = range Bots {
		if cur.Name != ref {
			continue
		}

		return cur, nil
	}

	err = fmt.Errorf("bot %s not found", ref)

	return nil, err
}

func BotCreate(payload Bot) (*Bot, error) {
	var cur *Bot

	var err error

	if payload.Name == "" {
		return nil, fmt.Errorf("bot name is required")
	}

	if !BotKindValid(payload.Kind) {
		return nil, fmt.Errorf("bot kind must be one of %v", BotKinds())
	}

	if payload.Token == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	for _, cur = range Bots {
		if cur.Name != payload.Name {
			continue
		}

		return nil, fmt.Errorf("bot %s already exists", payload.Name)
	}

	if payload.Agent != "" {
		_, err = AgentByName(payload.Agent)
		if err != nil {
			return nil, err
		}
	}

	payload.Id = uuid.NewString()
	payload.Enabled = true
	Bots = append(Bots, &payload)

	err = BotSave()
	if err != nil {
		return nil, err
	}

	return &payload, nil
}

func BotUpdateFields(ref string, name, token, agent, guildId *string, enabled *bool) error {
	var target *Bot
	var update Bot
	var index int

	var err error

	target, err = BotFind(ref)
	if err != nil {
		return err
	}

	update = *target

	if name != nil {
		if *name == "" {
			return fmt.Errorf("bot name cannot be empty")
		}

		update.Name = *name
	}

	if token != nil {
		if *token == "" {
			return fmt.Errorf("bot token cannot be empty")
		}

		update.Token = *token
	}

	if agent != nil {
		if *agent != "" {
			_, err = AgentByName(*agent)
			if err != nil {
				return err
			}
		}

		update.Agent = *agent
	}

	if guildId != nil {
		update.GuildId = *guildId
	}

	if enabled != nil {
		update.Enabled = *enabled
	}

	for index = range Bots {
		if Bots[index].Id != target.Id {
			continue
		}

		Bots[index] = &update
	}

	return BotSave()
}

func BotDelete(ref string) error {
	var target *Bot
	var cur *Bot
	var remaining []*Bot

	var err error

	target, err = BotFind(ref)
	if err != nil {
		return err
	}

	for _, cur = range Bots {
		if cur.Id == target.Id {
			continue
		}

		remaining = append(remaining, cur)
	}

	Bots = remaining

	return BotSave()
}

func BotsEnabled() []*Bot {
	var cur *Bot
	var running []*Bot

	for _, cur = range Bots {
		if !cur.Enabled {
			continue
		}

		running = append(running, cur)
	}

	return running
}
