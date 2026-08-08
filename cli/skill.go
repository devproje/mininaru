package main

import (
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var skillConfig *cobra.Command = &cobra.Command{
	Use:   "skill [list|show]",
	Short: "list installed skills or show what one sends to the model",
	Args:  cobra.MaximumNArgs(2),
	RunE:  skillExecute,
}

func skillList() error {
	var current modules.Skill

	for _, current = range modules.SkillAll() {
		fmt.Printf("%s\t%s\t%s\t%s\n", current.Name, current.Scope, current.Path, current.Description)
	}

	return nil
}

func skillShow(name string) error {
	var result string

	var err error

	result, err = modules.SkillResult(name, "")
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

func skillExecute(cmd *cobra.Command, args []string) error {
	var action string

	if len(args) == 0 {
		return skillList()
	}

	action = strings.ToLower(args[0])
	if action == "list" {
		return skillList()
	}

	if action != "show" {
		return fmt.Errorf("expected list or show")
	}

	if len(args) < 2 {
		return fmt.Errorf("show needs a skill name")
	}

	return skillShow(args[1])
}
