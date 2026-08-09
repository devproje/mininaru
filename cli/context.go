package main

import (
	"strconv"

	"github.com/devproje/mininaru/config"
	"github.com/spf13/cobra"
)

var contextConfig *cobra.Command = &cobra.Command{
	Use:   "context [max-chars]",
	Short: "show or set the approximate conversation context budget",
	Long: `Show the context budget, or set it by passing a new character count.

Older turns are dropped once a conversation grows past this budget. It is
measured in characters rather than tokens, so leave headroom for the model you
use. The minimum is 1024.`,
	Example: `  mininaru context
  mininaru context 120000`,
	Args: usageArgs(cobra.MaximumNArgs(1)),
	RunE: contextExecute,
}

func contextExecute(cmd *cobra.Command, args []string) error {
	var maxChars int

	var err error

	if len(args) == 0 {
		uiOk("%d", config.Client.Context.MaxChars)

		return nil
	}

	maxChars, err = strconv.Atoi(args[0])
	if err != nil || maxChars < 1024 {
		return usageErrorf("max-chars must be an integer of at least 1024")
	}

	config.Client.Context.MaxChars = maxChars
	err = config.ClientSave()
	if err != nil {
		return err
	}

	uiOk("%d", maxChars)

	return nil
}
