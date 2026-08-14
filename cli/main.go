// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

const latestSession = "@latest"

const (
	groupChat    = "chat"
	groupConfig  = "configuration"
	groupService = "service"
)

var (
	version string
	branch  string
	hash    string

	versionRef   bool
	sessionIdRef string
	resumeRef    string
	chatAgentRef string
	promptRef    string

	logLevelRef  string
	logFormatRef string
)

var root *cobra.Command = &cobra.Command{
	Use:   "mininaru [session id]",
	Short: "Lightweight LLM agent skeleton system",
	Long: `mininaru is a single binary that runs an LLM agent from the terminal.

Running it with no arguments opens the chat client with the global agent, either
resuming the session you name or starting a fresh one. The subcommands configure
providers, agents, tools and bots, and serve the OpenAI compatible API.`,
	Example: `  mininaru
  mininaru --resume
  mininaru -a reviewer -p "summarise the diff on stdin" -
  mininaru serve --port 8080`,
	SilenceUsage:      true,
	SilenceErrors:     true,
	Args:              usageArgs(cobra.MaximumNArgs(1)),
	PersistentPreRunE: bootstrapExecute,
	RunE:              execute,
}

func bootstrapExecute(cmd *cobra.Command, args []string) error {
	var err error

	if versionRef {
		resolveDataDir()

		return nil
	}

	err = bootstrap()
	if err != nil {
		return err
	}

	updateCheckStart(cmd)

	return nil
}

func dataDirPath() string {
	var path string

	path = os.Getenv("NARU_PATH")
	if path == "" {
		path = ".mininaru"
	}

	return path
}

func resolveDataDir() {
	var abs string

	var err error

	abs, err = filepath.Abs(dataDirPath())
	if err != nil {
		return
	}

	util.RootDir = abs
}

func bootstrap() error {
	var workingDir string

	var err error

	err = util.LogInit(util.LogOptions{Level: logLevelRef, Format: logFormatRef})
	if err != nil {
		return usageErrorf("init logging: %w", err)
	}

	workingDir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	err = modules.SetWorkingRoot(workingDir)
	if err != nil {
		return fmt.Errorf("set working root %s: %w", workingDir, err)
	}

	err = util.InitFS(dataDirPath())
	if err != nil {
		return fmt.Errorf("init data directory %s: %w", dataDirPath(), err)
	}

	util.DB, err = util.InitDatabase(util.Path("mininaru.db"))
	if err != nil {
		return fmt.Errorf("open database %s: %w", util.Path("mininaru.db"), err)
	}

	err = config.ClientInit()
	if err != nil {
		return fmt.Errorf("load client config: %w", err)
	}

	err = modules.WebLoad()
	if err != nil {
		return fmt.Errorf("load web search config: %w", err)
	}

	err = modules.SkillInit()
	if err != nil {
		return fmt.Errorf("load skills: %w", err)
	}

	err = core.ProviderInit()
	if err != nil {
		return fmt.Errorf("load providers: %w", err)
	}

	err = core.AgentInit()
	if err != nil {
		return fmt.Errorf("load agents: %w", err)
	}

	err = core.BotInit()
	if err != nil {
		return fmt.Errorf("load bots: %w", err)
	}

	core.InstallAgentTool()

	return nil
}

func showVersion() {
	var notice string

	fmt.Println()
	fmt.Println(util.NaruLogoWithPad("  "))
	fmt.Println()

	fmt.Println(util.RuntimeIdentity())

	notice = updateNotice()
	if notice != "" {
		fmt.Println(notice)
	}
}

func resolveAgent() (*core.NaruAgent, error) {
	if chatAgentRef != "" {
		return core.AgentByName(chatAgentRef)
	}

	if core.Global == nil {
		return nil, configErrorf("no agent configured, run `mininaru setup` or add one with `mininaru agent add`")
	}

	return core.Global, nil
}

func resolveSession(agent *core.NaruAgent, args []string) (*core.Session, error) {
	var id string
	var session *core.Session

	var err error

	id = sessionIdRef
	if id == "" {
		id = resumeRef
	}

	if id == latestSession && len(args) == 1 {
		id = args[0]
	}

	if id != "" && id != latestSession {
		session, err = core.SessionFind(id)
		if err != nil {
			return nil, err
		}
		if session.AgentId != agent.Id {
			return nil, fmt.Errorf("session %s belongs to agent %s, not %s", session.Id, session.AgentId, agent.Name)
		}

		return session, nil
	}

	if id == latestSession {
		session, err = core.SessionLatest(agent.Id)
		if err != nil {
			return nil, err
		}

		if session != nil {
			return session, nil
		}
	}

	return core.SessionCreate(agent, time.Now().Format("2006-01-02 15:04"))
}

func execute(cmd *cobra.Command, args []string) error {
	var agent *core.NaruAgent
	var content string
	var session *core.Session
	var history []*core.Message

	var err error

	if versionRef {
		showVersion()
		return nil
	}

	if config.Client.Tools.Enabled {
		err = withProgress(cmd.Context(), "connecting to mcp servers", func() error {
			return modules.MCPInit(cmd.Context())
		})
		if err != nil {
			return err
		}
	}

	agent, err = resolveAgent()
	if err != nil {
		return err
	}

	if promptRef != "" {
		content, err = promptContent(promptRef, os.Stdin)
		if err != nil {
			return err
		}

		if content == "" {
			return fmt.Errorf("prompt is empty")
		}
	}

	session, err = resolveSession(agent, args)
	if err != nil {
		return err
	}

	if content != "" {
		return runPrompt(cmd.Context(), os.Stdout, os.Stderr, session, agent, content)
	}

	history, err = core.MessageList(session.Id)
	if err != nil {
		return err
	}

	return runClient(session, agent, history)
}

func rootInit() {
	cobra.EnableTraverseRunHooks = true

	root.PersistentFlags().StringVar(&logLevelRef, "log-level", "",
		"diagnostic log level: "+strings.Join(util.LogLevels(), ", ")+" (default info, or "+util.LogLevelEnv+")")
	root.PersistentFlags().StringVar(&logFormatRef, "log-format", "",
		"diagnostic log format: "+strings.Join(util.LogFormats(), ", ")+" (default auto, or "+util.LogFormatEnv+")")

	root.PersistentFlags().BoolVar(&util.AppDebug, "debug", false, "enable debugging mode")
	root.PersistentFlags().BoolVar(&config.AllowDangerousTools, "allow-dangerous-tools", false, "allow file writes and shell commands for this run")

	root.Flags().BoolVar(&versionRef, "version", false, "check mininaru version info")
	root.Flags().StringVar(&sessionIdRef, "session", "", "resume chat with session id, omit the id for the latest session")
	root.Flags().Lookup("session").NoOptDefVal = latestSession

	root.Flags().StringVar(&resumeRef, "resume", "", "alias of --session")
	root.Flags().Lookup("resume").NoOptDefVal = latestSession

	root.Flags().StringVarP(&chatAgentRef, "agent", "a", "", "agent name or id to chat with, defaults to the global agent")
	root.Flags().StringVarP(&promptRef, "prompt", "p", "", "run one turn without the tui and print the answer, pass - to read it from stdin")

	root.SetFlagErrorFunc(usageFlagError)

	root.AddGroup(
		&cobra.Group{ID: groupChat, Title: "Chat:"},
		&cobra.Group{ID: groupConfig, Title: "Configuration:"},
		&cobra.Group{ID: groupService, Title: "Service:"},
	)

	session.GroupID = groupChat

	setup.GroupID = groupConfig
	provider.GroupID = groupConfig
	agent.GroupID = groupConfig
	thinking.GroupID = groupConfig
	contextConfig.GroupID = groupConfig
	toolsConfig.GroupID = groupConfig
	mcpConfig.GroupID = groupConfig
	skillConfig.GroupID = groupConfig
	webConfig.GroupID = groupConfig
	botConfig.GroupID = groupConfig

	serve.GroupID = groupService
	daemonConfig.GroupID = groupService
	updateCmd.GroupID = groupService

	root.AddCommand(setup)
	root.AddCommand(serve)
	root.AddCommand(provider)
	root.AddCommand(agent)
	root.AddCommand(session)
	root.AddCommand(thinking)
	root.AddCommand(contextConfig)
	root.AddCommand(toolsConfig)
	root.AddCommand(mcpConfig)
	root.AddCommand(skillConfig)
	root.AddCommand(webConfig)
	root.AddCommand(botConfig)
	root.AddCommand(daemonConfig)
	root.AddCommand(updateCmd)
}

func main() {
	var ctx context.Context
	var stop context.CancelFunc

	var err error

	if version != "" {
		util.AppVersion = version
	}

	if branch != "" {
		util.AppBranch = branch
	}

	if hash != "" {
		util.AppHash = hash
	}

	rootInit()

	ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = root.ExecuteContext(ctx)

	modules.MCPClose()

	if util.DB != nil {
		util.DB.Close()
	}

	if err != nil {
		reportError(err)
		os.Exit(exitCode(err))
	}
}
