// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/skill"
	"github.com/spf13/cobra"
)

var skillCmd *cobra.Command = &cobra.Command{
	Use:   "skill",
	Short: "inspect the skills mininaru found on disk",
	Long: `Inspect the skills mininaru found on disk.

A skill is a folder of instructions the agent can load on demand. Use show to
see the exact text a skill puts in front of the model.`,
	RunE: skillListExecute,
}

var skillListCmd *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list installed skills",
	RunE:    skillListExecute,
}

var skillShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <name>",
	Short: "print what a skill sends to the model",

	Args: cobra.ExactArgs(1),
	RunE: skillShowExecute,
}

var skillUsesCmd *cobra.Command = &cobra.Command{
	Use:     "uses",
	Aliases: []string{"stats"},
	Short:   "show which skills the model has loaded",
	RunE:    skillUsesExecute,
}

var skillUsesSessionRef string

func init() {
	skillUsesCmd.Flags().StringVar(&skillUsesSessionRef, "session", "", "only count loads from this session id")

	skillCmd.AddCommand(skillListCmd, skillShowCmd, skillUsesCmd)
}

func skillListExecute(cmd *cobra.Command, args []string) error {
	var all []skill.Skill
	var current skill.Skill

	all = skill.All()
	if len(all) == 0 {
		fmt.Println("no skills installed")
		return nil
	}

	for _, current = range all {
		fmt.Printf("%s  [%s]\n", current.Name, current.Scope)
		fmt.Printf("  path          %s\n", current.Path)
		fmt.Printf("  description   %s\n", current.Description)
	}

	return nil
}

func skillShowExecute(cmd *cobra.Command, args []string) error {
	var result string

	var err error

	result, err = skill.Result(args[0], "")
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

	var err error

	uses, err = core.SkillUseStats(skillUsesSessionRef)
	if err != nil {
		return err
	}

	if len(uses) == 0 {
		fmt.Println("no skill usage recorded")
		return nil
	}

	for _, current = range uses {
		scope = current.Scope
		if scope == "" {
			scope = "removed"
		}

		fmt.Printf("%s  [%s]  uses=%d  last=%s\n", current.Skill, scope, current.Count, current.LastUsed)
	}

	return nil
}
