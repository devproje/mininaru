package main

import (
	"fmt"
	"strings"

	"github.com/devproje/mininaru/config"
	"github.com/spf13/cobra"
)

var (
	thinkingShowRef bool
	thinkingHideRef bool
)

var thinking *cobra.Command = &cobra.Command{
	Use:   "thinking [off|low|medium|high|max]",
	Short: "show or set how hard the model thinks before answering",
	Args:  cobra.MaximumNArgs(1),
	RunE:  thinkingExecute,
}

func thinkingStatus() {
	var state string

	state = "hidden"
	if config.Client.Thinking.Show {
		state = "shown"
	}

	fmt.Printf("%s\t%s\n", config.Client.Thinking.Level, state)
}

func thinkingExecute(cmd *cobra.Command, args []string) error {
	var level string

	var err error

	if thinkingShowRef && thinkingHideRef {
		return fmt.Errorf("--show and --hide cannot be used together")
	}

	if len(args) == 0 && !thinkingShowRef && !thinkingHideRef {
		thinkingStatus()

		return nil
	}

	if len(args) == 1 {
		level = strings.ToLower(args[0])
		if !config.ThinkingValid(level) {
			return fmt.Errorf("invalid thinking level %q, expected one of %s",
				args[0], strings.Join(config.ThinkingLevels(), ", "))
		}

		config.Client.Thinking.Level = level
	}

	if thinkingShowRef {
		config.Client.Thinking.Show = true
	}

	if thinkingHideRef {
		config.Client.Thinking.Show = false
	}

	err = config.ClientSave()
	if err != nil {
		return err
	}

	thinkingStatus()

	return nil
}

func init() {
	thinking.Flags().BoolVar(&thinkingShowRef, "show", false, "show the thinking stream in the client")
	thinking.Flags().BoolVar(&thinkingHideRef, "hide", false, "keep the thinking stream hidden")
}
