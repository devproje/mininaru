// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

var (
	version string
	branch  string
	hash    string

	versionRef bool
)

func showVersion() {
	fmt.Println()
	fmt.Println(util.NaruLogoWithPad("  "))
	fmt.Println()

	fmt.Println(util.RuntimeIdentity())
}

func execute(cmd *cobra.Command, args []string) error {

	if versionRef {
		showVersion()
		return nil
	}
	return nil
}

var root *cobra.Command = &cobra.Command{
	RunE: execute,
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

	util.DB, err = util.NewDatabase(util.Path("data.db"))
	if err != nil {
		panic(err)
	}
	defer util.DB.Close()

	err = root.Execute()
	if err != nil {
		panic(err)
	}
}
