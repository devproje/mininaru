// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var skillConfig *cobra.Command = &cobra.Command{
	Use:   "skill",
	Short: "list installed skills or show what one sends to the model",
	Long: `Inspect the skills mininaru found on disk.

A skill is a folder of instructions the agent can load on demand. Use show to
see the exact text a skill puts in front of the model.`,
	Example: `  mininaru skill
  mininaru skill show code-review`,
	Args: usageArgs(cobra.NoArgs),
	RunE: skillListExecute,
}

var skillListCmd *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list installed skills",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    skillListExecute,
}

var skillShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <name>",
	Short: "print what a skill sends to the model",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  skillShowExecute,
}

func skillListExecute(cmd *cobra.Command, args []string) error {
	var all []modules.Skill
	var current modules.Skill
	var rows *uiRows

	all = modules.SkillAll()
	if len(all) == 0 {
		uiEmpty("no skills installed")

		return nil
	}

	rows = uiTable("NAME", "SCOPE", "PATH", "DESCRIPTION")

	for _, current = range all {
		rows.row(current.Name, current.Scope, current.Path, current.Description)
	}

	rows.flush()

	return nil
}

func skillShowExecute(cmd *cobra.Command, args []string) error {
	var result string

	var err error

	result, err = modules.SkillResult(args[0], "")
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

func init() {
	skillConfig.AddCommand(skillListCmd)
	skillConfig.AddCommand(skillShowCmd)
}
