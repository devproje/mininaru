// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

const narushAlias string = "narush"

var (
	version string
	branch  string
	hash    string

	versionRef bool
)

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

	if versionRef {
		showVersion()
		return nil
	}
	return nil
}

var root *cobra.Command = &cobra.Command{
	RunE:         execute,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		updateCheckStart(cmd)

		return nil
	},
}

func invokedAs(name string) bool {
	var base string

	base = filepath.Base(os.Args[0])
	base = strings.TrimSuffix(base, ".exe")

	return strings.EqualFold(base, name)
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

	root.Flags().BoolVar(&versionRef, "version", false, "checking mininaru version")

	root.AddCommand(serve)
	root.AddCommand(shellCmd)
	root.AddCommand(providerCmd)
	root.AddCommand(agentCmd)
	root.AddCommand(skillCmd)
	root.AddCommand(sessionCmd)
	root.AddCommand(updateCmd)

	util.DB, err = util.NewDatabase(util.Path("data.db"))
	if err != nil {
		panic(err)
	}
	defer util.DB.Close()

	if invokedAs(narushAlias) {
		os.Args = append([]string{os.Args[0], "shell"}, os.Args[1:]...)
	}

	err = root.Execute()
	if err != nil {
		os.Exit(1)
	}
}
