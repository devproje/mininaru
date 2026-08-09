package main

import (
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/spf13/cobra"
)

var webConfig *cobra.Command = &cobra.Command{
	Use:   "web",
	Short: "show or configure the web search provider",
	Long: `Configure the backend behind the web search tool.

The provider decides which endpoint and request shape mininaru uses; endpoint
and key override the defaults for that provider.`,
	Example: `  mininaru web
  mininaru web provider brave
  mininaru web key <api key>`,
	Args:              usageArgs(cobra.NoArgs),
	PersistentPreRunE: webLoadExecute,
	RunE:              webShowExecute,
}

var webShowCmd *cobra.Command = &cobra.Command{
	Use:   "show",
	Short: "show the current web search configuration",
	Args:  usageArgs(cobra.NoArgs),
	RunE:  webShowExecute,
}

var webProviderCmd *cobra.Command = &cobra.Command{
	Use:   "provider <name>",
	Short: "set the web search provider",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  webProviderExecute,
}

var webEndpointCmd *cobra.Command = &cobra.Command{
	Use:   "endpoint <url>",
	Short: "override the web search endpoint",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  webEndpointExecute,
}

var webKeyCmd *cobra.Command = &cobra.Command{
	Use:   "key <value>",
	Short: "set the web search api key",
	Args:  usageArgs(cobra.ExactArgs(1)),
	RunE:  webKeyExecute,
}

func webLoadExecute(cmd *cobra.Command, args []string) error {
	return modules.WebLoad()
}

func webShow() error {
	var cfg modules.SearchConfig
	var rows *uiRows

	cfg = modules.WebSearchConfig()

	rows = uiTable("PROVIDER", "ENDPOINT", "API KEY")
	rows.row(cfg.Provider, cfg.Endpoint, maskSecret(cfg.APIKey))
	rows.flush()

	return nil
}

func webShowExecute(cmd *cobra.Command, args []string) error {
	return webShow()
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

func webProviderExecute(cmd *cobra.Command, args []string) error {
	var cfg modules.SearchConfig

	cfg = modules.WebSearchConfig()
	cfg.Provider = strings.ToLower(strings.TrimSpace(args[0]))

	return webApply(cfg)
}

func webEndpointExecute(cmd *cobra.Command, args []string) error {
	var cfg modules.SearchConfig

	cfg = modules.WebSearchConfig()
	cfg.Endpoint = strings.TrimSpace(args[0])

	return webApply(cfg)
}

func webKeyExecute(cmd *cobra.Command, args []string) error {
	var cfg modules.SearchConfig

	cfg = modules.WebSearchConfig()
	cfg.APIKey = strings.TrimSpace(args[0])

	return webApply(cfg)
}

func init() {
	webConfig.AddCommand(webShowCmd)
	webConfig.AddCommand(webProviderCmd)
	webConfig.AddCommand(webEndpointCmd)
	webConfig.AddCommand(webKeyCmd)
}
