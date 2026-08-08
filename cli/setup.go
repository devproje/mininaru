package main

import (
	"fmt"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

var setup *cobra.Command = &cobra.Command{
	Use:   "setup",
	Short: "walk through the first run configuration",
	Args:  cobra.NoArgs,
	RunE:  setupExecute,
}

func setupProvider() (*core.Provider, error) {
	var reuse bool

	var err error

	if core.DefaultProvider != nil {
		fmt.Fprintf(askOut, "\nprovider %s already configured (%s)\n",
			core.DefaultProvider.Name, core.DefaultProvider.BaseURL)

		reuse, err = askConfirm("keep it", true)
		if err != nil {
			return nil, err
		}

		if reuse {
			return core.DefaultProvider, nil
		}
	}

	fmt.Fprintln(askOut, "\nan OpenAI-compatible endpoint to talk to")

	providerNameRef = ""
	providerBaseURLRef = ""
	providerApiKeyRef = ""

	err = providerAddAsk()
	if err != nil {
		return nil, err
	}

	core.ProviderCreate(core.Provider{
		Name:    providerNameRef,
		ApiKey:  providerApiKeyRef,
		BaseURL: providerBaseURLRef,
	})

	err = core.ProviderSave()
	if err != nil {
		return nil, err
	}

	return core.DefaultProvider, nil
}

func setupAgent(prov *core.Provider) error {
	var reuse bool
	var created *core.NaruAgent

	var err error

	if core.Global != nil {
		fmt.Fprintf(askOut, "\nglobal agent %s already configured (model %s)\n",
			core.Global.Name, core.Global.Model)

		reuse, err = askConfirm("keep it", true)
		if err != nil {
			return err
		}

		if reuse {
			return nil
		}
	}

	fmt.Fprintln(askOut, "\nthe agent the terminal client talks to")

	agentNameRef = ""
	agentModelRef = ""
	agentRoleRef = ""
	agentSoulRef = ""
	agentProviderRef = ""

	err = agentAddAsk()
	if err != nil {
		return err
	}

	if agentProviderRef != "" {
		prov, err = core.ProviderFind(agentProviderRef)
		if err != nil {
			return err
		}
	}

	created = core.AgentNew(agentNameRef, agentRoleRef, agentSoulRef, agentModelRef, prov)
	if created == nil {
		return fmt.Errorf("failed to create agent")
	}

	if core.Global != nil {
		core.Agents = append(core.Agents, core.Global)
	}

	core.Global = created

	return core.AgentSave()
}

func setupPreferences() error {
	var level string
	var tools bool

	var err error

	fmt.Fprintln(askOut)

	level, err = askChoice("thinking level", config.ThinkingLevels(), config.Client.Thinking.Level)
	if err != nil {
		return err
	}

	tools, err = askConfirm("enable tool calling", config.Client.Tools.Enabled)
	if err != nil {
		return err
	}

	config.Client.Thinking.Level = level
	config.Client.Tools.Enabled = tools

	return config.ClientSave()
}

func setupExecute(cmd *cobra.Command, args []string) error {
	var prov *core.Provider

	var err error

	if !askInteractive() {
		return fmt.Errorf("setup needs a terminal, configure with `provider add` and `agent add` instead")
	}

	fmt.Fprintf(askOut, "configuring mininaru in %s\n", util.RootDir)

	prov, err = setupProvider()
	if err != nil {
		return err
	}

	err = setupAgent(prov)
	if err != nil {
		return err
	}

	err = setupPreferences()
	if err != nil {
		return err
	}

	fmt.Fprintf(askOut, "\nready, run `mininaru` to start chatting with %s\n", core.Global.Name)

	return nil
}
