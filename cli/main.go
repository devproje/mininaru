// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/devproje/mininaru/modules/client"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

var (
	version string
	branch  string
	hash    string

	versionRef bool
	promptRef  string

	promptUrlRef     string
	promptSessionRef string
	promptAgentRef   string
	promptApiKeyRef  string
	promptFormatRef  string
)

var root *cobra.Command = &cobra.Command{
	RunE:         execute,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		updateCheckStart(cmd)

		return nil
	},
}

func showVersion() {
	var notice string

	fmt.Println()
	fmt.Println(util.NaruLogoWithPad("  "))
	fmt.Println()

	fmt.Println(util.RuntimeIdentity())

	notice = util.UpdateNotice()
	if notice != "" {
		fmt.Println(notice)
	}
}

func execute(cmd *cobra.Command, args []string) error {
	var err error
	if versionRef {
		showVersion()
		return nil
	}

	err = applyGateway(cmd)
	if err != nil {
		return err
	}

	if promptRef != "" {
		err = shortPrompt(promptRef)
		if err != nil {
			return err
		}

		return nil
	}

	err = clientExecute()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var path string
	var err error

	if version != "" {
		util.AppVersion = version
	}

	if branch != "" {
		util.AppBranch = branch
	}

	if hash != "" {
		util.AppHash = hash
	}

	path = os.Getenv("NARU_PATH")
	if path == "" {
		path = ".mininaru"
	}

	err = util.InitFS(path)
	if err != nil {
		panic(err)
	}

	err = util.NewLog(util.LogOptions{})
	if err != nil {
		panic(err)
	}

	root.Flags().BoolVar(&versionRef, "version", false, "checking mininaru version")
	root.Flags().StringVarP(&promptRef, "prompt", "p", "", "sending short stateless prompt")
	root.Flags().StringVar(&promptSessionRef, "session", "", "existing session id to prompt on")
	root.Flags().StringVar(&promptAgentRef, "agent", "", "agent name to open a new session with")
	root.Flags().StringVarP(&promptFormatRef, "format", "f", client.FormatString, "output format for -p: string|json|xml")
	root.PersistentFlags().StringVar(&promptUrlRef, "url", client.DefaultUrl, "websocket endpoint of the mininaru server")
	root.PersistentFlags().StringVar(&promptApiKeyRef, "api-key", "", "api key for the mininaru server")
	root.PersistentFlags().StringVar(&gatewayRef, "gateway", "", "named remote endpoint from 'mininaru gateway'")

	root.AddCommand(serve)
	root.AddCommand(gatewayCmd)
	root.AddCommand(daemonCmd)
	root.AddCommand(providerCmd)
	root.AddCommand(agentCmd)
	root.AddCommand(mcpCmd)
	root.AddCommand(skillCmd)
	root.AddCommand(sessionCmd)
	root.AddCommand(updateCmd)

	util.DB, err = util.NewDatabase(util.Path("data.db"))
	if err != nil {
		panic(err)
	}
	defer util.DB.Close()

	err = root.Execute()
	if err != nil {
		os.Exit(1)
	}
}
