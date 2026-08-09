// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var toolsConfig *cobra.Command = &cobra.Command{
	Use:   "tools",
	Short: "show, enable, disable, or list available tools",
	Long: `Control whether the agent may call tools at all.

With tools disabled the agent answers from the conversation alone and no MCP
server is contacted. Individual dangerous tools still need approval in the chat
client, or --allow-dangerous-tools for a single run.`,
	Example: `  mininaru tools
  mininaru tools list
  mininaru tools off`,
	Args: usageArgs(cobra.NoArgs),
	RunE: toolsStateExecute,
}

var toolsEnableCmd *cobra.Command = &cobra.Command{
	Use:     "enable",
	Aliases: []string{"on"},
	Short:   "let the agent call tools",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    toolsEnableExecute,
}

var toolsDisableCmd *cobra.Command = &cobra.Command{
	Use:     "disable",
	Aliases: []string{"off"},
	Short:   "stop the agent from calling tools",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    toolsDisableExecute,
}

var toolsListCmd *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list every tool the agent can call, including mcp tools",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    toolsListExecute,
}

func toolsState() string {
	if config.Client.Tools.Enabled {
		return "on"
	}

	return "off"
}

func toolsStateExecute(cmd *cobra.Command, args []string) error {
	var rows *uiRows

	rows = uiTable("TOOLS")
	rows.row(toolsState())
	rows.flush()

	return nil
}

func toolsToggle(enabled bool) error {
	var err error

	config.Client.Tools.Enabled = enabled

	err = config.ClientSave()
	if err != nil {
		return err
	}

	uiOk("tools %s", toolsState())

	return nil
}

func toolsEnableExecute(cmd *cobra.Command, args []string) error {
	return toolsToggle(true)
}

func toolsDisableExecute(cmd *cobra.Command, args []string) error {
	return toolsToggle(false)
}

func toolsListExecute(cmd *cobra.Command, args []string) error {
	var all []modules.Def
	var def modules.Def
	var rows *uiRows

	var err error

	err = withProgress(cmd.Context(), "connecting to mcp servers", func() error {
		return modules.MCPInit(cmd.Context())
	})
	if err != nil {
		return err
	}

	all = modules.DefaultTools()
	if len(all) == 0 {
		uiEmpty("no tools available")

		return nil
	}

	rows = uiTable("NAME", "PERMISSION", "SOURCE", "DESCRIPTION")

	for _, def = range all {
		rows.row(def.Name, def.Permission.String(), modules.ToolSource(def.Name), def.Description)
	}

	rows.flush()

	return nil
}

func init() {
	toolsConfig.AddCommand(toolsEnableCmd)
	toolsConfig.AddCommand(toolsDisableCmd)
	toolsConfig.AddCommand(toolsListCmd)
}
