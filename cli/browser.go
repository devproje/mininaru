// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"

	"github.com/devproje/mininaru/modules/browser"
	"github.com/spf13/cobra"
)

var browserCmd *cobra.Command = &cobra.Command{
	Use:   "browser",
	Short: "configure which installed browser the computer-use tools launch",
	Long: `Choose which installed browser the browser_* tools launch via chromedp.

Selection is by channel (chrome, chromium, edge, brave), not an exact path —
set MININARU_CHROME instead if you need to point at a specific binary.`,
	RunE: browserShowExecute,
}

var browserShowCmd *cobra.Command = &cobra.Command{
	Use:   "show",
	Short: "show the configured browser channel and resolved binary",
	RunE:  browserShowExecute,
}

var browserSetCmd *cobra.Command = &cobra.Command{
	Use:   "set <chrome|chromium|edge|brave>",
	Short: "pick which installed browser channel to launch",
	Args:  cobra.ExactArgs(1),
	RunE:  browserSetExecute,
}

var browserClearCmd *cobra.Command = &cobra.Command{
	Use:   "clear",
	Short: "reset to auto-detecting any installed browser",
	RunE:  browserClearExecute,
}

func init() {
	browserCmd.AddCommand(browserShowCmd, browserSetCmd, browserClearCmd)
}

func browserShowExecute(cmd *cobra.Command, args []string) error {
	var config browser.Config
	var channel string
	var path string

	var err error

	config, err = browser.LoadConfig()
	if err != nil {
		return err
	}

	channel = config.Channel
	if channel == "" {
		channel = "auto"
	}

	path = browser.ResolvedPath()
	if path == "" {
		path = "not found"
	}

	fmt.Printf("channel   %s\n", channel)
	fmt.Printf("resolved  %s\n", path)
	fmt.Printf("available %t\n", browser.Available())

	return nil
}

func browserSetExecute(cmd *cobra.Command, args []string) error {
	var channel string
	var config browser.Config

	var err error

	channel = args[0]
	if !browser.ValidChannel(channel) || channel == "" {
		return fmt.Errorf("unknown browser channel %q, want one of chrome, chromium, edge, brave", channel)
	}

	config, err = browser.LoadConfig()
	if err != nil {
		return err
	}

	config.Channel = channel

	err = browser.SaveConfig(config)
	if err != nil {
		return err
	}

	fmt.Printf("browser channel set to %s\n", channel)

	return nil
}

func browserClearExecute(cmd *cobra.Command, args []string) error {
	var config browser.Config

	var err error

	config, err = browser.LoadConfig()
	if err != nil {
		return err
	}

	config.Channel = ""

	err = browser.SaveConfig(config)
	if err != nil {
		return err
	}

	fmt.Println("browser channel reset to auto-detect")

	return nil
}
