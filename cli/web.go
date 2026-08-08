package main

import (
	"fmt"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var webConfig *cobra.Command = &cobra.Command{
	Use:   "web [show|provider|endpoint|key]",
	Short: "show or configure the web search provider",
	Args:  cobra.MaximumNArgs(2),
	RunE:  webExecute,
}

func webShow() error {
	var cfg modules.SearchConfig

	cfg = modules.WebSearchConfig()

	fmt.Printf("%s\t%s\t%s\n", cfg.Provider, cfg.Endpoint, maskSecret(cfg.APIKey))

	return nil
}

func webApply(cfg modules.SearchConfig) error {
	var err error

	err = modules.WebValidate(&cfg)
	if err != nil {
		return err
	}

	modules.WebSetSearch(cfg)

	err = modules.WebSave()
	if err != nil {
		return err
	}

	return webShow()
}

func webExecute(cmd *cobra.Command, args []string) error {
	var action string
	var cfg modules.SearchConfig

	var err error

	err = modules.WebLoad()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return webShow()
	}

	action = strings.ToLower(args[0])
	if action == "show" {
		return webShow()
	}

	if len(args) < 2 {
		return fmt.Errorf("%s needs a value", action)
	}

	cfg = modules.WebSearchConfig()

	switch action {
	case "provider":
		cfg.Provider = strings.ToLower(strings.TrimSpace(args[1]))
		return webApply(cfg)
	case "endpoint":
		cfg.Endpoint = strings.TrimSpace(args[1])
		return webApply(cfg)
	case "key":
		cfg.APIKey = strings.TrimSpace(args[1])
		return webApply(cfg)
	}

	return fmt.Errorf("expected show, provider, endpoint, or key")
}
