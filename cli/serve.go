// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/devproje/mininaru/bot"
	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/server"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

const apiKeyEnv = "MININARU_API_KEY"

var (
	serveHostRef   string
	servePortRef   int
	serveApiKeyRef string
)

var serve *cobra.Command = &cobra.Command{
	Use:   "serve",
	Short: "serve the api and any configured bot front ends",
	Long: `Run the OpenAI compatible HTTP API, plus every enabled bot.

The API is stateless and exposes each agent as a model name. Requests need the
bearer token from --api-key or ` + "`" + apiKeyEnv + "`" + `, and only safe tools are offered.
Send SIGHUP to reload configuration without restarting.`,
	Example: `  mininaru serve
  mininaru serve --host 0.0.0.0 --port 8080`,
	Args: usageArgs(cobra.NoArgs),
	RunE: serveExecute,
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
			util.Log.Info("reloading configuration on SIGHUP")

			err = modules.WebReload()
			if err != nil {
				util.Log.Error("web reload failed", "error", err)
			}

			err = modules.SkillReload()
			if err != nil {
				util.Log.Error("skill reload failed", "error", err)
			}

			if config.Client.Tools.Enabled {
				err = modules.MCPReload(ctx)
				if err != nil {
					util.Log.Error("mcp reload failed", "error", err)
				}
			}

			err = registry.Reload()
			if err != nil {
				util.Log.Error("agent registry reload failed", "error", err)
				continue
			}

			util.Log.Info("reload complete", "agents", len(registry.List()))
			server.AnnounceAgents(registry)
		case <-ctx.Done():
			return
		}
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

	discord, err = bot.NewDiscord(bot.DiscordConfig{Token: cfg.Token, BotId: cfg.Id, Agent: cfg.Agent, GuildId: cfg.GuildId}, registry)
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
	var cur *core.Bot
	var running *bot.Discord
	var started []*bot.Discord

	var err error

	for _, cur = range core.BotsEnabled() {
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
		return configErrorf("api key is required, pass --api-key or set %s", apiKeyEnv)
	}

	if config.Client.Tools.Enabled {
		err = withProgress(cmd.Context(), "connecting to mcp servers", func() error {
			return modules.MCPInit(cmd.Context())
		})
		if err != nil {
			return err
		}

		defer modules.MCPClose()
	}

	registry = core.NewRegistry()

	err = registry.Reload()
	if err != nil {
		return err
	}

	if len(registry.List()) == 0 {
		return configErrorf("no agent configured, run `mininaru setup` or add one with `mininaru agent add`")
	}

	go watchReload(cmd.Context(), registry)

	started, err = startBots(registry)
	if err != nil {
		return err
	}

	defer stopBots(started)

	uiNote("serving %d agent(s) on http://%s:%d", len(registry.List()), cfg.Host, cfg.Port)

	return server.Serve(cmd.Context(), cfg, registry)
}

func init() {
	serve.Flags().StringVar(&serveHostRef, "host", server.DefaultHost, "address to bind the api server")
	serve.Flags().IntVar(&servePortRef, "port", server.DefaultPort, "port to bind the api server")
	serve.Flags().StringVar(&serveApiKeyRef, "api-key", "", "bearer token required by api clients, defaults to "+apiKeyEnv)
}
