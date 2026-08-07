package main

import (
	"fmt"
	"strconv"

	"github.com/devproje/mininaru/config"
	"github.com/spf13/cobra"
)

var contextConfig *cobra.Command = &cobra.Command{
	Use:   "context [max-chars]",
	Short: "show or set the approximate conversation context budget",
	Args:  cobra.MaximumNArgs(1),
	RunE:  contextExecute,
}

func contextExecute(cmd *cobra.Command, args []string) error {
	var maxChars int
	var err error

	if len(args) == 0 {
		fmt.Println(config.Client.Context.MaxChars)
		return nil
	}

	maxChars, err = strconv.Atoi(args[0])
	if err != nil || maxChars < 1024 {
		return fmt.Errorf("max-chars must be an integer of at least 1024")
	}

	config.Client.Context.MaxChars = maxChars
	err = config.ClientSave()
	if err != nil {
		return err
	}

	fmt.Println(maxChars)
	return nil
}
