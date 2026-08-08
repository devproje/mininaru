package main

import (
	"fmt"
	"strings"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var toolsConfig *cobra.Command = &cobra.Command{
	Use:   "tools [on|off|list]",
	Short: "show, enable, disable, or list available tools",
	Args:  cobra.MaximumNArgs(1),
	RunE:  toolsExecute,
}

func toolsExecute(cmd *cobra.Command, args []string) error {
	var state string
	var def modules.Def

	var err error

	if len(args) == 0 {
		state = "off"
		if config.Client.Tools.Enabled {
			state = "on"
		}
		fmt.Println(state)
		return nil
	}

	state = strings.ToLower(args[0])
	if state == "list" {
		err = modules.MCPInit(cmd.Context())
		if err != nil {
			return err
		}

		for _, def = range modules.DefaultTools() {
			fmt.Printf("%s\t%s\t%s\t%s\n", def.Name, def.Permission.String(), modules.ToolSource(def.Name), def.Description)
		}
		return nil
	}
	if state != "on" && state != "off" {
		return fmt.Errorf("expected on, off, or list")
	}

	config.Client.Tools.Enabled = state == "on"
	err = config.ClientSave()
	if err != nil {
		return err
	}

	fmt.Println(state)
	return nil
}
