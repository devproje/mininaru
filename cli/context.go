// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

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

Once a conversation grows past this budget its oldest turns leave the replayed
history. They are summarised into the conversation first unless compaction is
off, in which case they are dropped. The budget is measured in characters rather
than tokens, so leave headroom for the model you use. The minimum is 1024.`,
	Example: `  mininaru context
  mininaru context 120000
  mininaru context compact off`,
	Args: usageArgs(cobra.MaximumNArgs(1)),
	RunE: contextExecute,
}

var contextCompactCmd *cobra.Command = &cobra.Command{
	Use:   "compact [on|off]",
	Short: "show or set whether older turns are summarised before they leave",
	Long: `Show whether compaction is on, or turn it on or off.

With compaction on, the turns that fall out of the context budget are summarised
into a running summary carried in the system prompt, which costs one extra model
call on the turn where something falls out. With it off they are dropped and
what they contained is gone. A summary already saved for a conversation keeps
being used either way.`,
	Example: `  mininaru context compact
  mininaru context compact off`,
	Args: usageArgs(cobra.MaximumNArgs(1)),
	RunE: contextCompactExecute,
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

func compactState() string {
	if config.Client.Context.Compact {
		return "on"
	}

	return "off"
}

func contextCompactExecute(cmd *cobra.Command, args []string) error {
	var rows *uiRows

	var err error

	if len(args) == 0 {
		rows = uiTable("COMPACT")
		rows.row(compactState())
		rows.flush()

		return nil
	}

	switch args[0] {
	case "on", "enable", "true":
		config.Client.Context.Compact = true
	case "off", "disable", "false":
		config.Client.Context.Compact = false
	default:
		return usageErrorf("compact must be on or off")
	}

	err = config.ClientSave()
	if err != nil {
		return err
	}

	uiOk("compact %s", compactState())

	return nil
}

func init() {
	contextConfig.AddCommand(contextCompactCmd)
}
