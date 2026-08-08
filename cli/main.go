package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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

var (
	version string
	branch  string
	hash    string

	versionRef   bool
	sessionIdRef string
	chatAgentRef string
	promptRef    string

	logLevelRef  string
	logFormatRef string
)

var root *cobra.Command = &cobra.Command{
	Use:               "mininaru [session id]",
	Short:             "Lightweight LLM agent skeleton system",
	SilenceUsage:      true,
	Args:              cobra.MaximumNArgs(1),
	PersistentPreRunE: bootstrapExecute,
	RunE:              execute,
}

func bootstrapExecute(cmd *cobra.Command, args []string) error {
	if versionRef {
		return nil
	}

	return bootstrap()
}

func bootstrap() error {
	var workingDir string
	var path string

	var err error

	err = util.LogInit(util.LogOptions{Level: logLevelRef, Format: logFormatRef})
	if err != nil {
		return err
	}

	workingDir, err = os.Getwd()
	if err != nil {
		return err
	}

	err = modules.SetWorkingRoot(workingDir)
	if err != nil {
		return err
	}

	path = os.Getenv("NARU_PATH")
	if path == "" {
		path = ".mininaru"
	}

	err = util.InitFS(path)
	if err != nil {
		return err
	}

	util.DB, err = util.InitDatabase(util.Path("mininaru.db"))
	if err != nil {
		return err
	}

	err = config.ClientInit()
	if err != nil {
		return err
	}

	err = modules.WebLoad()
	if err != nil {
		return err
	}

	err = modules.SkillInit()
	if err != nil {
		return err
	}

	err = core.ProviderInit()
	if err != nil {
		return err
	}

	err = core.AgentInit()
	if err != nil {
		return err
	}

	return core.BotInit()
}

func showVersion() {
	fmt.Println()
	fmt.Println(util.NaruLogoWithPad("  "))
	fmt.Println()

	fmt.Println(util.RuntimeIdentity())
}

func resolveAgent() (*core.NaruAgent, error) {
	if chatAgentRef != "" {
		return core.AgentByName(chatAgentRef)
	}

	if core.Global == nil {
		return nil, fmt.Errorf("no agent configured, please configure a provider and an agent first")
	}

	return core.Global, nil
}

func resolveSession(agent *core.NaruAgent, args []string) (*core.Session, error) {
	var id string
	var session *core.Session

	var err error

	id = sessionIdRef
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
		err = modules.MCPInit(cmd.Context())
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

func main() {
	var ctx context.Context
	var stop context.CancelFunc

	var err error

	util.AppVersion = version
	util.AppBranch = branch
	util.AppHash = hash

	root.PersistentFlags().StringVar(&logLevelRef, "log-level", "",
		"diagnostic log level: "+strings.Join(util.LogLevels(), ", ")+" (default info, or "+util.LogLevelEnv+")")
	root.PersistentFlags().StringVar(&logFormatRef, "log-format", "",
		"diagnostic log format: "+strings.Join(util.LogFormats(), ", ")+" (default auto, or "+util.LogFormatEnv+")")

	root.Flags().BoolVar(&util.AppDebug, "debug", false, "enable debugging mode")
	root.Flags().BoolVar(&config.AllowDangerousTools, "allow-dangerous-tools", false, "allow file writes and shell commands for this run")
	root.Flags().BoolVar(&versionRef, "version", false, "check mininaru version info")
	root.Flags().StringVar(&sessionIdRef, "session", "", "resume chat with session id, omit the id for the latest session")
	root.Flags().Lookup("session").NoOptDefVal = latestSession

	root.Flags().StringVar(&sessionIdRef, "resume", "", "alias of --session")
	root.Flags().Lookup("resume").NoOptDefVal = latestSession

	root.Flags().StringVarP(&chatAgentRef, "agent", "a", "", "agent name or id to chat with, defaults to the global agent")
	root.Flags().StringVarP(&promptRef, "prompt", "p", "", "run one turn without the tui and print the answer, pass - to read it from stdin")

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

	ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = root.ExecuteContext(ctx)

	modules.MCPClose()

	if util.DB != nil {
		util.DB.Close()
	}

	if err != nil {
		os.Exit(1)
	}
}
