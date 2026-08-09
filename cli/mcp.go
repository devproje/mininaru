package main

import (
	"fmt"
	"strconv"

	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var mcpConfig *cobra.Command = &cobra.Command{
	Use:   "mcp",
	Short: "show or manage configured mcp servers",
	Long: `Manage the MCP servers mininaru connects to for extra tools.

Every server is connected on startup, so a slow or unreachable one delays each
run until its timeout expires. Disable a server instead of removing it to keep
its configuration around.`,
	Example: `  mininaru mcp
  mininaru mcp add files --stdio npx --arg -y --arg @modelcontextprotocol/server-filesystem
  mininaru mcp disable files`,
	Args:              usageArgs(cobra.NoArgs),
	PersistentPreRunE: mcpLoadExecute,
	RunE:              mcpListExecute,
}

var mcpListCmd *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list configured mcp servers and their connection state",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    mcpListExecute,
}

var mcpAddCmd *cobra.Command = &cobra.Command{
	Use:   "add <name>",
	Short: "add an mcp server",
	Long: `Add an MCP server under the given name.

Pass --stdio to launch a local command, or --url for a streamable HTTP endpoint.
On a terminal, omitting both prompts for the transport and its settings.`,
	Example: `  mininaru mcp add files --stdio npx --arg -y --arg @modelcontextprotocol/server-filesystem
  mininaru mcp add remote --url https://example.com/mcp --header Authorization="Bearer token"`,
	Args: usageArgs(cobra.ExactArgs(1)),
	RunE: mcpAddExecute,
}

var mcpRemoveCmd *cobra.Command = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "remove an mcp server",
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    mcpRemoveExecute,
}

var mcpEnableCmd *cobra.Command = &cobra.Command{
	Use:   "enable <name>",
	Short: "enable an mcp server",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  mcpEnableExecute,
}

var mcpDisableCmd *cobra.Command = &cobra.Command{
	Use:   "disable <name>",
	Short: "disable an mcp server without removing its configuration",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  mcpDisableExecute,
}

var (
	mcpCommandRef    string
	mcpArgsRef       []string
	mcpEnvRef        map[string]string
	mcpDirRef        string
	mcpUrlRef        string
	mcpHeaderRef     map[string]string
	mcpNoDaemonRef   bool
	mcpPermissionRef string
	mcpTimeoutRef    int
)

func mcpFind(name string) int {
	var index int

	for index = range modules.MCP.Servers {
		if modules.MCP.Servers[index].Name != name {
			continue
		}

		return index
	}

	return -1
}

func mcpLoadExecute(cmd *cobra.Command, args []string) error {
	return modules.MCPLoad()
}

func mcpStatusOf(all []modules.MCPStatus, name string) (modules.MCPStatus, bool) {
	var index int

	for index = range all {
		if all[index].Name != name {
			continue
		}

		return all[index], true
	}

	return modules.MCPStatus{}, false
}

func mcpListExecute(cmd *cobra.Command, args []string) error {
	var all []modules.MCPStatus
	var status modules.MCPStatus
	var entry modules.MCPServer
	var rows *uiRows
	var state string
	var tools string
	var known bool

	var err error

	if len(modules.MCP.Servers) == 0 {
		uiEmpty("no mcp servers yet, add one with `mininaru mcp add <name> --stdio <command>`")

		return nil
	}

	err = withProgress(cmd.Context(), "connecting to mcp servers", func() error {
		return modules.MCPInit(cmd.Context())
	})
	if err != nil {
		return err
	}

	all = modules.MCPStatusAll()

	rows = uiTable("NAME", "TRANSPORT", "STATE", "TOOLS", "ERROR")

	for _, entry = range modules.MCP.Servers {
		status, known = mcpStatusOf(all, entry.Name)

		state = "disabled"
		tools = "-"

		if known {
			state = "failed"
			if status.Connected {
				state = "connected"
			}

			tools = strconv.Itoa(status.Tools)
		}

		rows.row(entry.Name, entry.Transport, state, tools, status.Error)
	}

	rows.flush()

	return nil
}

func mcpAddAsk() error {
	var transport string

	var err error

	if mcpCommandRef != "" || mcpUrlRef != "" {
		return nil
	}

	transport, err = askChoice("transport", []string{modules.TransportStdio, modules.TransportHTTP}, modules.TransportStdio)
	if err != nil {
		return err
	}

	if transport == modules.TransportHTTP {
		mcpUrlRef, err = askRequired("endpoint url")

		return err
	}

	mcpCommandRef, err = askRequired("command")
	if err != nil {
		return err
	}

	mcpDirRef, err = askText("working directory", "")

	return err
}

func mcpAdd(name string) error {
	var entry modules.MCPServer
	var daemon bool

	var err error

	if mcpFind(name) >= 0 {
		return fmt.Errorf("mcp server %q already exists", name)
	}

	if askInteractive() {
		err = mcpAddAsk()
		if err != nil {
			return err
		}
	}

	entry.Name = name
	entry.Command = mcpCommandRef
	entry.Args = mcpArgsRef
	entry.Env = mcpEnvRef
	entry.Dir = mcpDirRef
	entry.URL = mcpUrlRef
	entry.Headers = mcpHeaderRef
	entry.Permission = mcpPermissionRef
	entry.TimeoutSeconds = mcpTimeoutRef

	entry.Transport = modules.TransportStdio
	if mcpCommandRef == "" {
		entry.Transport = modules.TransportHTTP
	}

	if mcpNoDaemonRef {
		daemon = false
		entry.Daemon = &daemon
	}

	err = modules.MCPValidate(&entry)
	if err != nil {
		return err
	}

	modules.MCP.Servers = append(modules.MCP.Servers, entry)

	err = modules.MCPSave()
	if err != nil {
		return err
	}

	uiOk("added mcp server %s", name)

	return nil
}

func mcpRemove(name string) error {
	var index int

	var err error

	index = mcpFind(name)
	if index < 0 {
		return fmt.Errorf("mcp server %q not found", name)
	}

	modules.MCP.Servers = append(modules.MCP.Servers[:index], modules.MCP.Servers[index+1:]...)

	err = modules.MCPSave()
	if err != nil {
		return err
	}

	uiOk("removed mcp server %s", name)

	return nil
}

func mcpToggleLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}

	return "disabled"
}

func mcpToggle(name string, enabled bool) error {
	var index int

	var err error

	index = mcpFind(name)
	if index < 0 {
		return fmt.Errorf("mcp server %q not found", name)
	}

	modules.MCP.Servers[index].Enabled = &enabled

	err = modules.MCPSave()
	if err != nil {
		return err
	}

	uiOk("%s mcp server %s", mcpToggleLabel(enabled), name)

	return nil
}

func mcpAddExecute(cmd *cobra.Command, args []string) error {
	return mcpAdd(args[0])
}

func mcpRemoveExecute(cmd *cobra.Command, args []string) error {
	return mcpRemove(args[0])
}

func mcpEnableExecute(cmd *cobra.Command, args []string) error {
	return mcpToggle(args[0], true)
}

func mcpDisableExecute(cmd *cobra.Command, args []string) error {
	return mcpToggle(args[0], false)
}

func init() {
	mcpAddCmd.Flags().StringVar(&mcpCommandRef, "stdio", "", "command to run for a stdio mcp server")
	mcpAddCmd.Flags().StringArrayVar(&mcpArgsRef, "arg", nil, "argument for the stdio command, repeatable")
	mcpAddCmd.Flags().StringToStringVar(&mcpEnvRef, "env", nil, "extra environment variable for the stdio command, repeatable")
	mcpAddCmd.Flags().StringVar(&mcpDirRef, "dir", "", "working directory for the stdio command")
	mcpAddCmd.Flags().StringVar(&mcpUrlRef, "url", "", "endpoint of a streamable http mcp server")
	mcpAddCmd.Flags().StringToStringVar(&mcpHeaderRef, "header", nil, "extra http header, repeatable")
	mcpAddCmd.Flags().BoolVar(&mcpNoDaemonRef, "no-daemon", false, "hide this server from the api server and bots")
	mcpAddCmd.Flags().StringVar(&mcpPermissionRef, "permission", "", "force safe or dangerous for every tool of this server")
	mcpAddCmd.Flags().IntVar(&mcpTimeoutRef, "timeout", 0, "seconds to wait while connecting, defaults to 10")

	mcpConfig.AddCommand(mcpListCmd)
	mcpConfig.AddCommand(mcpAddCmd)
	mcpConfig.AddCommand(mcpRemoveCmd)
	mcpConfig.AddCommand(mcpEnableCmd)
	mcpConfig.AddCommand(mcpDisableCmd)
}
