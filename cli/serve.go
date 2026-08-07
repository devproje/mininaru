package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/devproje/mininaru/bot"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/server"
	"github.com/spf13/cobra"
)

const apiKeyEnv = "MININARU_API_KEY"

const discordTokenEnv = "MININARU_DISCORD_TOKEN"

var (
	serveHostRef         string
	servePortRef         int
	serveApiKeyRef       string
	serveDiscordTokenRef string
	serveDiscordAgentRef string
	serveDiscordGuildRef string
)

var serve *cobra.Command = &cobra.Command{
	Use:   "serve",
	Short: "serve the api and any configured bot front ends",
	RunE:  serveExecute,
}

func watchReload(ctx context.Context, registry *core.Registry) {
	var hangups chan os.Signal

	var err error

	hangups = make(chan os.Signal, 1)
	signal.Notify(hangups, syscall.SIGHUP)
	defer signal.Stop(hangups)

	for {
		select {
		case <-hangups:
			err = registry.Reload()
			if err != nil {
				fmt.Fprintf(os.Stderr, "reload failed: %v\n", err)
				continue
			}

			fmt.Println("reloaded agents on SIGHUP")
			server.AnnounceAgents(registry)
		case <-ctx.Done():
			return
		}
	}
}

func overrideBot() *core.Bot {
	var token string

	token = serveDiscordTokenRef
	if token == "" {
		token = os.Getenv(discordTokenEnv)
	}

	if token == "" {
		return nil
	}

	return &core.Bot{
		Name:    "discord",
		Kind:    core.BotDiscord,
		Token:   token,
		Agent:   serveDiscordAgentRef,
		GuildId: serveDiscordGuildRef,
		Enabled: true,
	}
}

func startBot(cfg *core.Bot, registry *core.Registry) (*bot.Discord, error) {
	var discord *bot.Discord

	var err error

	if cfg.Kind != core.BotDiscord {
		return nil, fmt.Errorf("bot %s has unsupported kind %q", cfg.Name, cfg.Kind)
	}

	if cfg.Agent != "" {
		_, err = registry.Get(cfg.Agent)
		if err != nil {
			return nil, fmt.Errorf("bot %s: %w", cfg.Name, err)
		}
	}

	discord, err = bot.NewDiscord(bot.DiscordConfig{Token: cfg.Token, Agent: cfg.Agent, GuildId: cfg.GuildId}, registry)
	if err != nil {
		return nil, fmt.Errorf("bot %s: %w", cfg.Name, err)
	}

	err = discord.Start()
	if err != nil {
		return nil, fmt.Errorf("bot %s: %w", cfg.Name, err)
	}

	return discord, nil
}

func startBots(registry *core.Registry) ([]*bot.Discord, error) {
	var configured []*core.Bot
	var override *core.Bot
	var started []*bot.Discord
	var running *bot.Discord
	var cur *core.Bot

	var err error

	override = overrideBot()
	if override != nil {
		configured = []*core.Bot{override}
	}

	if override == nil {
		configured = core.BotsEnabled()
	}

	for _, cur = range configured {
		running, err = startBot(cur, registry)
		if err != nil {
			stopBots(started)

			return nil, err
		}

		started = append(started, running)
	}

	return started, nil
}

func stopBots(started []*bot.Discord) {
	var cur *bot.Discord

	for _, cur = range started {
		cur.Stop()
	}
}

func serveExecute(cmd *cobra.Command, args []string) error {
	var cfg server.Config
	var registry *core.Registry
	var started []*bot.Discord

	var err error

	cfg = server.Config{Host: serveHostRef, Port: servePortRef, ApiKey: serveApiKeyRef}
	if cfg.ApiKey == "" {
		cfg.ApiKey = os.Getenv(apiKeyEnv)
	}

	if cfg.ApiKey == "" {
		return fmt.Errorf("api key is required, pass --api-key or set %s", apiKeyEnv)
	}

	registry = core.NewRegistry()

	err = registry.Reload()
	if err != nil {
		return err
	}

	if len(registry.List()) == 0 {
		return fmt.Errorf("no agent configured, please configure a provider and an agent first")
	}

	go watchReload(cmd.Context(), registry)

	started, err = startBots(registry)
	if err != nil {
		return err
	}

	defer stopBots(started)

	return server.Serve(cmd.Context(), cfg, registry)
}

func init() {
	serve.Flags().StringVar(&serveHostRef, "host", server.DefaultHost, "address to bind the api server")
	serve.Flags().IntVar(&servePortRef, "port", server.DefaultPort, "port to bind the api server")
	serve.Flags().StringVar(&serveApiKeyRef, "api-key", "", "bearer token required by api clients, defaults to "+apiKeyEnv)

	serve.Flags().StringVar(&serveDiscordTokenRef, "discord-token", "", "start the discord bot with this token, defaults to "+discordTokenEnv)
	serve.Flags().StringVar(&serveDiscordAgentRef, "discord-agent", "", "agent new discord channels talk to, defaults to the global agent")
	serve.Flags().StringVar(&serveDiscordGuildRef, "discord-guild", "", "register slash commands to this guild only, which applies them instantly")
}
