// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/devproje/mininaru/cli/shell"
	"github.com/devproje/mininaru/util"
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
	shellApiKeyRef  string
	shellVersionRef bool
)

func init() {
	shellCmd.Flags().StringVar(&shellUrlRef, "url", shell.DEFAULT_URL, "websocket endpoint of the mininaru server")
	shellCmd.Flags().StringVar(&shellSessionRef, "session", "", "existing session id to chat on")
	shellCmd.Flags().StringVar(&shellAgentRef, "agent", "", "agent name to open a new session with")
	shellCmd.Flags().StringVar(&shellApiKeyRef, "api-key", "", "api key for the mininaru server")

	shellCmd.Flags().BoolVar(&shellVersionRef, "version", false, "checking narush version")
}

func isLoopbackURL(endpoint string) bool {
	var parsed *url.URL
	var host string

	var err error

	parsed, err = url.Parse(endpoint)
	if err != nil {
		return false
	}

	host = parsed.Hostname()

	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func resolveApiKey(explicit string, endpoint string) string {
	var fromEnv string
	var fromFile []byte

	var err error

	if explicit != "" {
		return explicit
	}

	fromEnv = os.Getenv("MININARU_API_KEY")
	if fromEnv != "" {
		return fromEnv
	}

	if !isLoopbackURL(endpoint) {
		return ""
	}

	fromFile, err = os.ReadFile(util.Path("mininaru.key"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(fromFile))
}

func showShellVersion() {
	var base string

	base = util.RuntimeIdentity()
	base = strings.ReplaceAll(base, "mininaru", "narush")

	fmt.Printf("%s\n", base)
}

func shellExecute(cmd *cobra.Command, args []string) error {
	var apiKey string

	if shellVersionRef {
		showShellVersion()
		return nil
	}

	apiKey = resolveApiKey(shellApiKeyRef, shellUrlRef)

	return shell.Run(shell.Options{Url: shellUrlRef, Session: shellSessionRef, Agent: shellAgentRef, ApiKey: apiKey})
}
