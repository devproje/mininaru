// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strconv"

	"github.com/devproje/mininaru/modules/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd *cobra.Command = &cobra.Command{
	Use:   "mcp",
	Short: "manage mcp servers",
	Long: `Manage the MCP servers mininaru connects to for extra tools.

A running "mininaru serve" reloads its own connections on SIGHUP, so changes
made here take effect without a restart.`,
	RunE: mcpListExecute,
}

var mcpListCmd *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list configured mcp servers and their connection state",
	RunE:    mcpListExecute,
}

var mcpShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <name>",
	Short: "show one mcp server's configuration and connection state",
	Args:  cobra.ExactArgs(1),
	RunE:  mcpShowExecute,
}

var mcpAddCmd *cobra.Command = &cobra.Command{
	Use:   "add <name>",
	Short: "add an mcp server",
	Long: `Add an MCP server under the given name.

Pass --stdio to launch a local command, or --url for a streamable HTTP endpoint.`,
	Example: `  mininaru mcp add files --stdio npx --arg -y --arg @modelcontextprotocol/server-filesystem
  mininaru mcp add remote --url https://example.com/mcp --header Authorization="Bearer token"`,
	Args: cobra.ExactArgs(1),
	RunE: mcpAddExecute,
}

var mcpRemoveCmd *cobra.Command = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "remove an mcp server",
	Args:    cobra.ExactArgs(1),
	RunE:    mcpRemoveExecute,
}

var mcpEnableCmd *cobra.Command = &cobra.Command{
	Use:   "enable <name>",
	Short: "enable an mcp server",
	Args:  cobra.ExactArgs(1),
	RunE:  mcpEnableExecute,
}

var mcpDisableCmd *cobra.Command = &cobra.Command{
	Use:   "disable <name>",
	Short: "disable an mcp server without removing its configuration",
	Args:  cobra.ExactArgs(1),
	RunE:  mcpDisableExecute,
}

var (
	mcpAddStdioRef          string
	mcpAddArgsRef           []string
	mcpAddEnvRef            map[string]string
	mcpAddDirRef            string
	mcpAddUrlRef            string
	mcpAddHeaderRef         map[string]string
	mcpAddPermissionRef     string
	mcpAddTimeoutRef        int
	mcpAddToolPermissionRef map[string]string
)

func init() {
	mcpAddCmd.Flags().StringVar(&mcpAddStdioRef, "stdio", "", "command to run for a stdio mcp server")
	mcpAddCmd.Flags().StringArrayVar(&mcpAddArgsRef, "arg", nil, "argument for the stdio command, repeatable")
	mcpAddCmd.Flags().StringToStringVar(&mcpAddEnvRef, "env", nil, "extra environment variable for the stdio command, repeatable")
	mcpAddCmd.Flags().StringVar(&mcpAddDirRef, "dir", "", "working directory for the stdio command")
	mcpAddCmd.Flags().StringVar(&mcpAddUrlRef, "url", "", "endpoint of a streamable http mcp server")
	mcpAddCmd.Flags().StringToStringVar(&mcpAddHeaderRef, "header", nil, "extra http header, repeatable")
	mcpAddCmd.Flags().StringVar(&mcpAddPermissionRef, "permission", "", "force safe or dangerous for every tool of this server")
	mcpAddCmd.Flags().IntVar(&mcpAddTimeoutRef, "timeout", 0, "seconds to wait while connecting, defaults to 10")
	mcpAddCmd.Flags().StringToStringVar(&mcpAddToolPermissionRef, "tool-permission", nil, "force safe or dangerous for one tool by name (tool=safe|dangerous), repeatable")

	mcpCmd.AddCommand(mcpListCmd, mcpShowCmd, mcpAddCmd, mcpRemoveCmd, mcpEnableCmd, mcpDisableCmd)
}

func mcpFind(name string) int {
	var index int

	for index = range mcp.Loaded.Servers {
		if mcp.Loaded.Servers[index].Name == name {
			return index
		}
	}

	return -1
}

func mcpStatusOf(all []mcp.Status, name string) mcp.Status {
	var index int

	for index = range all {
		if all[index].Name == name {
			return all[index]
		}
	}

	return mcp.Status{Name: name}
}

func mcpState(status mcp.Status) string {
	if !status.Enabled {
		return "disabled"
	}
	if status.Connected {
		return "connected"
	}

	return "failed"
}

func printMcpRow(entry mcp.Server, status mcp.Status) {
	var tools string

	tools = "-"
	if status.Enabled {
		tools = strconv.Itoa(status.Tools)
	}

	fmt.Printf("%-16s %-10s %-10s %-6s %s\n", entry.Name, entry.Transport, mcpState(status), tools, status.Error)
}

func mcpListExecute(cmd *cobra.Command, args []string) error {
	var all []mcp.Status
	var entry mcp.Server

	var err error

	err = mcp.Load()
	if err != nil {
		return err
	}

	if len(mcp.Loaded.Servers) == 0 {
		fmt.Println("no mcp servers configured")
		return nil
	}

	fmt.Println("connecting to mcp servers...")

	err = mcp.Init(cmd.Context())
	if err != nil {
		return err
	}
	defer mcp.Close()

	all = mcp.StatusAll()

	fmt.Printf("%-16s %-10s %-10s %-6s %s\n", "NAME", "TRANSPORT", "STATE", "TOOLS", "ERROR")
	for _, entry = range mcp.Loaded.Servers {
		printMcpRow(entry, mcpStatusOf(all, entry.Name))
	}

	return nil
}

func printMcpServer(entry mcp.Server, status mcp.Status) {
	var key string
	var value string

	fmt.Printf("%s  [%s]\n", entry.Name, mcpState(status))
	fmt.Printf("  transport      %s\n", entry.Transport)

	if entry.Transport == mcp.TransportStdio {
		fmt.Printf("  command        %s %s\n", entry.Command, entry.Args)
		if entry.Dir != "" {
			fmt.Printf("  dir            %s\n", entry.Dir)
		}
	} else {
		fmt.Printf("  url            %s\n", entry.URL)
	}

	for key, value = range entry.Env {
		fmt.Printf("  env            %s=%s\n", key, value)
	}
	for key, value = range entry.Headers {
		fmt.Printf("  header         %s=%s\n", key, value)
	}
	if entry.TimeoutSeconds > 0 {
		fmt.Printf("  timeout        %ds\n", entry.TimeoutSeconds)
	}
	if entry.Permission != "" {
		fmt.Printf("  permission     %s\n", entry.Permission)
	}
	for key, value = range entry.ToolPermission {
		fmt.Printf("  tool_permission %s=%s\n", key, value)
	}

	fmt.Printf("  tools          %d\n", status.Tools)
	if status.Error != "" {
		fmt.Printf("  error          %s\n", status.Error)
	}
}

func mcpShowExecute(cmd *cobra.Command, args []string) error {
	var index int

	var err error

	err = mcp.Load()
	if err != nil {
		return err
	}

	index = mcpFind(args[0])
	if index < 0 {
		return fmt.Errorf("mcp server %q not found", args[0])
	}

	fmt.Println("connecting to mcp server...")

	err = mcp.Init(cmd.Context())
	if err != nil {
		return err
	}
	defer mcp.Close()

	printMcpServer(mcp.Loaded.Servers[index], mcpStatusOf(mcp.StatusAll(), args[0]))

	return nil
}

func mcpAdd(name string) error {
	var entry mcp.Server

	var err error

	err = mcp.Load()
	if err != nil {
		return err
	}

	if mcpFind(name) >= 0 {
		return fmt.Errorf("mcp server %q already exists", name)
	}

	entry.Name = name
	entry.Command = mcpAddStdioRef
	entry.Args = mcpAddArgsRef
	entry.Env = mcpAddEnvRef
	entry.Dir = mcpAddDirRef
	entry.URL = mcpAddUrlRef
	entry.Headers = mcpAddHeaderRef
	entry.Permission = mcpAddPermissionRef
	entry.TimeoutSeconds = mcpAddTimeoutRef
	entry.ToolPermission = mcpAddToolPermissionRef

	entry.Transport = mcp.TransportStdio
	if mcpAddStdioRef == "" {
		entry.Transport = mcp.TransportHTTP
	}

	err = mcp.Validate(&entry)
	if err != nil {
		return err
	}

	mcp.Loaded.Servers = append(mcp.Loaded.Servers, entry)

	err = mcp.Save()
	if err != nil {
		return err
	}

	fmt.Printf("added mcp server %s\n", name)

	return nil
}

func mcpAddExecute(cmd *cobra.Command, args []string) error {
	return mcpAdd(args[0])
}

func mcpRemove(name string) error {
	var index int

	var err error

	err = mcp.Load()
	if err != nil {
		return err
	}

	index = mcpFind(name)
	if index < 0 {
		return fmt.Errorf("mcp server %q not found", name)
	}

	mcp.Loaded.Servers = append(mcp.Loaded.Servers[:index], mcp.Loaded.Servers[index+1:]...)

	err = mcp.Save()
	if err != nil {
		return err
	}

	fmt.Printf("removed mcp server %s\n", name)

	return nil
}

func mcpRemoveExecute(cmd *cobra.Command, args []string) error {
	return mcpRemove(args[0])
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

	err = mcp.Load()
	if err != nil {
		return err
	}

	index = mcpFind(name)
	if index < 0 {
		return fmt.Errorf("mcp server %q not found", name)
	}

	mcp.Loaded.Servers[index].Enabled = &enabled

	err = mcp.Save()
	if err != nil {
		return err
	}

	fmt.Printf("%s mcp server %s\n", mcpToggleLabel(enabled), name)

	return nil
}

func mcpEnableExecute(cmd *cobra.Command, args []string) error {
	return mcpToggle(args[0], true)
}

func mcpDisableExecute(cmd *cobra.Command, args []string) error {
	return mcpToggle(args[0], false)
}
