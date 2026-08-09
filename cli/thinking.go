// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"

	"github.com/devproje/mininaru/config"
	"github.com/spf13/cobra"
)

var (
	thinkingShowRef bool
	thinkingHideRef bool
)

var thinking *cobra.Command = &cobra.Command{
	Use:   "thinking [off|low|medium|high|max]",
	Short: "show or set how hard the model thinks before answering",
	Long: `Show the reasoning effort, or set it by naming a level.

Higher levels let the model reason longer before answering, which costs more
tokens and time. --show and --hide control whether the chat client renders the
reasoning stream, independent of the level.`,
	Example: `  mininaru thinking
  mininaru thinking high --show`,
	ValidArgs: config.ThinkingLevels(),
	Args:      usageArgs(cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs)),
	RunE:      thinkingExecute,
}

func thinkingStatus() {
	var state string
	var rows *uiRows

	state = "hidden"
	if config.Client.Thinking.Show {
		state = "shown"
	}

	rows = uiTable("LEVEL", "STREAM")
	rows.row(config.Client.Thinking.Level, state)
	rows.flush()
}

func thinkingExecute(cmd *cobra.Command, args []string) error {
	var level string

	var err error

	if thinkingShowRef && thinkingHideRef {
		return usageErrorf("--show and --hide cannot be used together")
	}

	if len(args) == 0 && !thinkingShowRef && !thinkingHideRef {
		thinkingStatus()

		return nil
	}

	if len(args) == 1 {
		level = strings.ToLower(args[0])
		if !config.ThinkingValid(level) {
			return usageErrorf("invalid thinking level %q, expected one of %s",
				args[0], strings.Join(config.ThinkingLevels(), ", "))
		}

		config.Client.Thinking.Level = level
	}

	if thinkingShowRef {
		config.Client.Thinking.Show = true
	}

	if thinkingHideRef {
		config.Client.Thinking.Show = false
	}

	err = config.ClientSave()
	if err != nil {
		return err
	}

	thinkingStatus()

	return nil
}

func init() {
	thinking.Flags().BoolVar(&thinkingShowRef, "show", false, "show the thinking stream in the client")
	thinking.Flags().BoolVar(&thinkingHideRef, "hide", false, "keep the thinking stream hidden")
}
