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

var skillNotesCmd *cobra.Command = &cobra.Command{
	Use:   "notes <name>",
	Short: "show the lessons recorded against a skill",
	Long: `List what the agent learned while using a skill.

Pending notes are shown to the model every time the skill is loaded. Notes
become applied once skill_revise folds them into the instructions.`,
	Example: `  mininaru skill notes code-review
  mininaru skill notes code-review --all`,
	Args: usageArgs(cobra.ExactArgs(1)),
	RunE: skillNotesExecute,
}

var skillRevisionsCmd *cobra.Command = &cobra.Command{
	Use:     "revisions <name>",
	Aliases: []string{"history"},
	Short:   "show earlier versions of a skill",
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    skillRevisionsExecute,
}

var skillRollbackCmd *cobra.Command = &cobra.Command{
	Use:   "rollback <name> [revision id]",
	Short: "restore a skill to an earlier version",
	Long: `Put back a previous version of a skill.

Without a revision id the most recent snapshot is restored. The version being
replaced is itself snapshotted first, so a rollback can be undone.`,
	Example: `  mininaru skill rollback code-review
  mininaru skill rollback code-review 4f1c2a9e-...`,
	Args: usageArgs(cobra.RangeArgs(1, 2)),
	RunE: skillRollbackExecute,
}

var (
	skillUsesSessionRef string
	skillNotesAllRef    bool
)

func skillListExecute(cmd *cobra.Command, args []string) error {
	var all []modules.Skill
	var current modules.Skill
	var rows *uiRows

	if activeServerAddress() != "" {
		return remoteSkillListExecute(cmd.Context())
	}

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

	if activeServerAddress() != "" {
		return remoteSkillShowExecute(cmd.Context(), args[0])
	}

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

func skillNotesExecute(cmd *cobra.Command, args []string) error {
	var notes []modules.SkillNote
	var current modules.SkillNote
	var state string
	var rows *uiRows

	var err error

	notes, err = modules.SkillNotesFor(args[0], !skillNotesAllRef)
	if err != nil {
		return err
	}

	if len(notes) == 0 {
		if skillNotesAllRef {
			uiEmpty("no notes recorded for %s", args[0])
		} else {
			uiEmpty("no pending notes for %s, pass --all to include applied ones", args[0])
		}

		return nil
	}

	rows = uiTable("RECORDED", "STATE", "NOTE")

	for _, current = range notes {
		state = "pending"
		if current.Applied {
			state = "applied"
		}

		rows.row(current.CreatedAt, state, current.Note)
	}

	rows.flush()

	return nil
}

func skillRevisionsExecute(cmd *cobra.Command, args []string) error {
	var revisions []modules.SkillRevision
	var current modules.SkillRevision
	var reason string
	var rows *uiRows

	var err error

	revisions, err = modules.SkillRevisions(args[0])
	if err != nil {
		return err
	}

	if len(revisions) == 0 {
		uiEmpty("no revision history for %s", args[0])

		return nil
	}

	rows = uiTable("ID", "REPLACED AT", "REASON")

	for _, current = range revisions {
		reason = current.Reason
		if reason == "" {
			reason = "-"
		}

		rows.row(current.Id, current.CreatedAt, reason)
	}

	rows.flush()

	return nil
}

func skillRollbackExecute(cmd *cobra.Command, args []string) error {
	var revision string
	var target string

	var err error

	if len(args) == 2 {
		revision = args[1]
	}

	target, err = modules.SkillRestore(args[0], revision)
	if err != nil {
		return err
	}

	fmt.Printf("restored %s from its revision history\n%s\n", args[0], target)

	return nil
}

func init() {
	skillUsesCmd.Flags().StringVar(&skillUsesSessionRef, "session", "", "only count loads from this session id")
	skillNotesCmd.Flags().BoolVar(&skillNotesAllRef, "all", false, "include notes already folded into the skill")

	skillConfig.AddCommand(skillListCmd)
	skillConfig.AddCommand(skillShowCmd)
	skillConfig.AddCommand(skillUsesCmd)
	skillConfig.AddCommand(skillNotesCmd)
	skillConfig.AddCommand(skillRevisionsCmd)
	skillConfig.AddCommand(skillRollbackCmd)
}
