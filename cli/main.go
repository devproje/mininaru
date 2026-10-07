// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

var (
	version string
	branch  string
	hash    string

	versionRef bool

	cwdRef string
)

func execute(cmd *cobra.Command, args []string) error {
	var br string
	var arch string

	if versionRef {
		if version != branch {
			br = fmt.Sprintf(" (%s)", branch)
		}

		arch = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("mininaru %s-%s%s %s\n", version, hash, br, arch)

		return nil
	}

	return nil
}

var root *cobra.Command = &cobra.Command{
	Use:  "mininaru",
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
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	err = util.NewLog(util.LogOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	root.Flags().BoolVar(&util.AppDebug, "debug", false, "mininaru debugging mode")
	root.Flags().BoolVar(&versionRef, "version", false, "checking mininaru version")
	root.PersistentFlags().StringVarP(&cwdRef, "cwd", "C", ".", "change target directory (default: \".\")")

	root.AddCommand(serve)

	util.DB, err = util.NewDatabase(util.Path("state.db"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer util.DB.Close()

	err = root.Execute()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
