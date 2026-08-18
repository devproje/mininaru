// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

var setup *cobra.Command = &cobra.Command{
	Use:   "setup",
	Short: "walk through the first run configuration",
	Long: `Choose a paired client or server installation in one guided pass.

Client mode verifies the remote server fingerprint and requests pairing. Server
mode configures a provider, an agent and local defaults. Existing configuration
is offered back as the default, so this is safe to re-run.`,
	Example: `  mininaru setup`,
	Args:    usageArgs(cobra.NoArgs),
	RunE:    setupExecute,
}

var setupPair = pairWithServer

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

func setupApiKey() (string, error) {
	var key string
	var raw [24]byte

	var err error

	key, err = askSecret("api key for the http api, empty to generate one", true)
	if err != nil {
		return "", err
	}

	if key != "" {
		return key, nil
	}

	_, err = rand.Read(raw[:])
	if err != nil {
		return "", err
	}

	key = hex.EncodeToString(raw[:])
	fmt.Fprintf(askOut, "generated api key: %s\n", key)

	return key, nil
}

func setupEnvFile(path string) error {
	var key string

	var err error

	err = daemonEnvValid(path)
	if err == nil {
		return nil
	}

	_, err = os.Stat(path)
	if err == nil {
		return fmt.Errorf("%s exists but does not define %s, fix it and run `mininaru daemon install`", path, apiKeyEnv)
	}
	if !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(askOut, "the daemon reads %s from %s\n", apiKeyEnv, path)

	key, err = setupApiKey()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0700)
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(path, []byte(apiKeyEnv+"="+key+"\n"), 0600)
}

func setupDaemon(cmd *cobra.Command) error {
	var install bool
	var envFile string

	var err error

	_, err = exec.LookPath("systemctl")
	if err != nil {
		return nil
	}

	fmt.Fprintln(askOut, "\nthe daemon runs the http api and any configured bots in the background")

	install, err = askConfirm("install the systemd user daemon", false)
	if err != nil || !install {
		return err
	}

	_, envFile, err = daemonPaths()
	if err != nil {
		return err
	}

	err = setupEnvFile(envFile)
	if err != nil {
		return err
	}

	return daemonInstallExecute(cmd, nil)
}

func setupClient(cmd *cobra.Command) error {
	var address string
	var name string

	var err error

	fmt.Fprintln(askOut, "\npair this device with a mininaru server")

	address, err = askRequired("server address (host:port)")
	if err != nil {
		return err
	}

	name, err = askText("device name", pairDeviceName())
	if err != nil {
		return err
	}

	return setupPair(cmd.Context(), address, name, "")
}

func setupServer(cmd *cobra.Command) error {
	var prov *core.Provider

	var err error

	config.Client.Mode = config.ModeServer

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

	err = setupDaemon(cmd)
	if err != nil {
		fmt.Fprintf(askOut, "\ndaemon setup skipped: %v\n", err)
		fmt.Fprintln(askOut, "run `mininaru daemon install` once it is sorted out")
	}

	fmt.Fprintf(askOut, "\nready, run `mininaru` to start chatting with %s\n", core.Global.Name)

	return nil
}

func setupExecute(cmd *cobra.Command, args []string) error {
	var mode string

	var err error

	if !askInteractive() {
		return usageErrorf("setup needs a terminal, configure the server with `provider add` and `agent add`, or pair a client with `mininaru pair`")
	}

	fmt.Fprintf(askOut, "configuring mininaru in %s\n\n", util.RootDir)

	mode, err = askChoice("mode", []string{"server", "client"}, "server")
	if err != nil {
		return err
	}

	if mode == "client" {
		return setupClient(cmd)
	}

	return setupServer(cmd)
}
