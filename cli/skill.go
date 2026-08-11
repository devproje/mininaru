// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strconv"

	"github.com/devproje/mininaru/core"
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

var skillUsesCmd *cobra.Command = &cobra.Command{
	Use:     "uses",
	Aliases: []string{"stats"},
	Short:   "show which skills the model has loaded",
	Example: `  mininaru skill uses
  mininaru skill uses --session 4f1c2a9e`,
	Args: usageArgs(cobra.NoArgs),
	RunE: skillUsesExecute,
}

var skillUsesSessionRef string

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

func skillUsesExecute(cmd *cobra.Command, args []string) error {
	var uses []*core.SkillUse
	var current *core.SkillUse
	var scope string
	var rows *uiRows

	var err error

	uses, err = core.SkillUseStats(skillUsesSessionRef)
	if err != nil {
		return err
	}

	if len(uses) == 0 {
		uiEmpty("no skill usage recorded")

		return nil
	}

	rows = uiTable("NAME", "SCOPE", "USES", "LAST USED")

	for _, current = range uses {
		scope = current.Scope
		if scope == "" {
			scope = "removed"
		}

		rows.row(current.Skill, scope, strconv.Itoa(current.Count), current.LastUsed)
	}

	rows.flush()

	return nil
}

func init() {
	skillUsesCmd.Flags().StringVar(&skillUsesSessionRef, "session", "", "only count loads from this session id")

	skillConfig.AddCommand(skillListCmd)
	skillConfig.AddCommand(skillShowCmd)
	skillConfig.AddCommand(skillUsesCmd)
}
