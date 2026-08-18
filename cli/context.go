// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/devproje/mininaru/config"
	"github.com/spf13/cobra"
)

var contextConfig *cobra.Command = &cobra.Command{
	Use:   "context",
	Short: "show context management settings",
	Long: `Show context management settings.

The TUI reads actual input-token usage from provider responses. When the
provider exposes its model context window, mininaru uses that value for the
status display and automatic compaction threshold.`,
	Example: `  mininaru context
	  mininaru context compact off`,
	Args: usageArgs(cobra.NoArgs),
	RunE: contextExecute,
}

var contextCompactCmd *cobra.Command = &cobra.Command{
	Use:   "compact [on|off]",
	Short: "show or set whether older turns are summarised before they leave",
	Long: `Show whether compaction is on, or turn it on or off.

With compaction on, completed turns are summarised into a running summary when
provider-reported input usage reaches 90% of a known model context window. This
costs one extra model call. With it off mininaru does not compact automatically
or enforce a separate local history limit. A summary already saved for a
conversation keeps being used either way.`,
	Example: `  mininaru context compact
  mininaru context compact off`,
	Args: usageArgs(cobra.MaximumNArgs(1)),
	RunE: contextCompactExecute,
}

func contextExecute(cmd *cobra.Command, args []string) error {
	uiOk("compact %s", compactState())

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
