package main

import (
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var mcpConfig *cobra.Command = &cobra.Command{
	Use:   "mcp [list|add|remove|enable|disable]",
	Short: "show or manage configured mcp servers",
	Args:  cobra.MaximumNArgs(2),
	RunE:  mcpExecute,
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

func mcpList(cmd *cobra.Command) error {
	var status modules.MCPStatus
	var state string

	var err error

	err = modules.MCPInit(cmd.Context())
	if err != nil {
		return err
	}

	for _, status = range modules.MCPStatusAll() {
		state = "connected"
		if !status.Connected {
			state = "failed"
		}

		fmt.Printf("%s\t%s\t%s\t%d\t%s\n", status.Name, status.Transport, state, status.Tools, status.Error)
	}

	return nil
}

func mcpAdd(name string) error {
	var entry modules.MCPServer
	var daemon bool

	var err error

	if mcpFind(name) >= 0 {
		return fmt.Errorf("mcp server %q already exists", name)
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

	fmt.Println(name)
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

	fmt.Println(name)
	return nil
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

	fmt.Println(name)
	return nil
}

func mcpExecute(cmd *cobra.Command, args []string) error {
	var action string

	var err error

	err = modules.MCPLoad()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return mcpList(cmd)
	}

	action = strings.ToLower(args[0])
	if action == "list" {
		return mcpList(cmd)
	}

	if len(args) < 2 {
		return fmt.Errorf("%s needs a server name", action)
	}

	switch action {
	case "add":
		return mcpAdd(args[1])
	case "remove":
		return mcpRemove(args[1])
	case "enable":
		return mcpToggle(args[1], true)
	case "disable":
		return mcpToggle(args[1], false)
	}

	return fmt.Errorf("expected list, add, remove, enable, or disable")
}

func init() {
	mcpConfig.Flags().StringVar(&mcpCommandRef, "stdio", "", "command to run for a stdio mcp server")
	mcpConfig.Flags().StringArrayVar(&mcpArgsRef, "arg", nil, "argument for the stdio command, repeatable")
	mcpConfig.Flags().StringToStringVar(&mcpEnvRef, "env", nil, "extra environment variable for the stdio command, repeatable")
	mcpConfig.Flags().StringVar(&mcpDirRef, "dir", "", "working directory for the stdio command")
	mcpConfig.Flags().StringVar(&mcpUrlRef, "url", "", "endpoint of a streamable http mcp server")
	mcpConfig.Flags().StringToStringVar(&mcpHeaderRef, "header", nil, "extra http header, repeatable")
	mcpConfig.Flags().BoolVar(&mcpNoDaemonRef, "no-daemon", false, "hide this server from the api server and bots")
	mcpConfig.Flags().StringVar(&mcpPermissionRef, "permission", "", "force safe or dangerous for every tool of this server")
	mcpConfig.Flags().IntVar(&mcpTimeoutRef, "timeout", 0, "seconds to wait while connecting, defaults to 10")
}
