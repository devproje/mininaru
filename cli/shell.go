// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/devproje/mininaru/cli/shell"
	"github.com/spf13/cobra"
)

var shellCmd *cobra.Command = &cobra.Command{
	Use:   "shell",
	Short: "interactive bash and agent shell",
	Long:  "Interactive shell that switches between a bash prompt and an agent chat with Shift+Tab",

	Example: fmt.Sprintf("  mininaru shell\n  mininaru shell --url %s", shell.DEFAULT_URL),
	RunE:    shellExecute,
}

var (
	shellUrlRef     string
	shellSessionRef string
	shellAgentRef   string
)

func init() {
	shellCmd.Flags().StringVar(&shellUrlRef, "url", shell.DEFAULT_URL, "websocket endpoint of the mininaru server")
	shellCmd.Flags().StringVar(&shellSessionRef, "session", "", "existing session id to chat on")
	shellCmd.Flags().StringVar(&shellAgentRef, "agent", "", "agent name to open a new session with")
}

func shellExecute(cmd *cobra.Command, args []string) error {
	return shell.Run(shell.Options{Url: shellUrlRef, Session: shellSessionRef, Agent: shellAgentRef})
}
