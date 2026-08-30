// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var providerCmd *cobra.Command = &cobra.Command{
	Use:   "provider",
	Short: "manage model providers",
}

var providerAddCmd *cobra.Command = &cobra.Command{
	Use:   "add <name>",
	Short: "register a new provider",

	Args: cobra.ExactArgs(1),
	RunE: providerAddExecute,
}

var providerListCmd *cobra.Command = &cobra.Command{
	Use:   "list",
	Short: "list registered providers",
	RunE:  providerListExecute,
}

var providerShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <id-or-name>",
	Short: "show a provider",

	Args: cobra.ExactArgs(1),
	RunE: providerShowExecute,
}

var providerSetCmd *cobra.Command = &cobra.Command{
	Use:   "set <id-or-name>",
	Short: "update a provider",

	Args: cobra.ExactArgs(1),
	RunE: providerSetExecute,
}

var providerRemoveCmd *cobra.Command = &cobra.Command{
	Use:     "remove <id-or-name>",
	Aliases: []string{"rm", "delete"},
	Short:   "remove a provider",

	Args: cobra.ExactArgs(1),
	RunE: providerRemoveExecute,
}

var providerActivateCmd *cobra.Command = &cobra.Command{
	Use:   "activate <id-or-name>",
	Short: "make a provider the active one",

	Args: cobra.ExactArgs(1),
	RunE: providerActivateExecute,
}

var (
	providerAddApiKeyRef   string
	providerAddBaseUrlRef  string
	providerAddActivateRef bool

	providerSetNameRef    string
	providerSetApiKeyRef  string
	providerSetBaseUrlRef string
)

func init() {
	providerAddCmd.Flags().StringVar(&providerAddApiKeyRef, "api-key", "", "API key for the provider")
	providerAddCmd.Flags().StringVar(&providerAddBaseUrlRef, "base-url", "", "base URL of the provider's OpenAI-compatible endpoint")
	providerAddCmd.Flags().BoolVar(&providerAddActivateRef, "activate", false, "activate the provider immediately")

	providerSetCmd.Flags().StringVar(&providerSetNameRef, "name", "", "new name for the provider")
	providerSetCmd.Flags().StringVar(&providerSetApiKeyRef, "api-key", "", "new API key for the provider")
	providerSetCmd.Flags().StringVar(&providerSetBaseUrlRef, "base-url", "", "new base URL for the provider")

	providerCmd.AddCommand(providerAddCmd, providerListCmd, providerShowCmd, providerSetCmd, providerRemoveCmd, providerActivateCmd)
}

func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}

	if len(secret) <= 8 {
		return "****"
	}

	return secret[:4] + "..." + secret[len(secret)-4:]
}

func resolveProvider(idOrName string) (*core.Provider, error) {
	var prov *core.Provider
	var list []*core.Provider
	var item *core.Provider

	var err error

	prov, err = core.ProviderRead(idOrName)
	if err == nil {
		return prov, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	list, err = core.ProviderList()
	if err != nil {
		return nil, err
	}

	for _, item = range list {
		if item.Name == idOrName {
			return item, nil
		}
	}

	return nil, fmt.Errorf("provider %q not found", idOrName)
}

func printProvider(prov *core.Provider) {
	var active string

	active = ""
	if prov.Active {
		active = "  (active)"
	}

	fmt.Printf("%s  %s%s\n", prov.Id, prov.Name, active)
	fmt.Printf("  base_url  %s\n", prov.BaseUrl)
	fmt.Printf("  api_key   %s\n", maskSecret(prov.ApiKey))
}

func providerAddExecute(cmd *cobra.Command, args []string) error {
	var prov core.Provider

	var err error

	prov = core.Provider{Id: uuid.NewString(), Name: args[0], ApiKey: providerAddApiKeyRef, BaseUrl: providerAddBaseUrlRef}

	err = core.ProviderCreate(&prov)
	if err != nil {
		return err
	}

	if providerAddActivateRef {
		err = core.ProviderActivate(prov.Id)
		if err != nil {
			return err
		}
	}

	fmt.Printf("provider %s created (%s)\n", prov.Name, prov.Id)

	return nil
}

func providerListExecute(cmd *cobra.Command, args []string) error {
	var remote bool
	var list []*core.Provider
	var prov *core.Provider

	var err error

	remote, err = remoteGet(cmd, "/providers", &list)
	if err != nil {
		return err
	}

	if !remote {
		list, err = core.ProviderList()
		if err != nil {
			return err
		}
	}

	if len(list) == 0 {
		fmt.Println("no providers registered")
		return nil
	}

	for _, prov = range list {
		printProvider(prov)
	}

	return nil
}

func providerShowExecute(cmd *cobra.Command, args []string) error {
	var remote bool
	var list []*core.Provider
	var item *core.Provider
	var prov *core.Provider

	var err error

	remote, err = remoteGet(cmd, "/providers", &list)
	if err != nil {
		return err
	}

	if !remote {
		prov, err = resolveProvider(args[0])
		if err != nil {
			return err
		}

		printProvider(prov)

		return nil
	}

	for _, item = range list {
		if item.Id == args[0] || item.Name == args[0] {
			printProvider(item)

			return nil
		}
	}

	return fmt.Errorf("provider %q not found", args[0])
}

func providerSetExecute(cmd *cobra.Command, args []string) error {
	var prov *core.Provider

	var err error

	prov, err = resolveProvider(args[0])
	if err != nil {
		return err
	}

	err = core.ProviderUpdate(prov.Id, &core.Provider{Name: providerSetNameRef, ApiKey: providerSetApiKeyRef, BaseUrl: providerSetBaseUrlRef})
	if err != nil {
		return err
	}

	fmt.Printf("provider %s updated\n", prov.Id)

	return nil
}

func providerRemoveExecute(cmd *cobra.Command, args []string) error {
	var id string
	var remote bool
	var prov *core.Provider

	var err error

	id, remote, err = remoteResolveId(cmd, "/providers", args[0])
	if err != nil {
		return err
	}

	if remote {
		_, err = remoteDo(cmd, http.MethodDelete, "/providers/"+id)
		if err != nil {
			return err
		}

		fmt.Printf("provider %s removed\n", id)

		return nil
	}

	prov, err = resolveProvider(args[0])
	if err != nil {
		return err
	}

	err = core.ProviderDelete(prov.Id)
	if err != nil {
		return err
	}

	fmt.Printf("provider %s removed\n", prov.Id)

	return nil
}

func providerActivateExecute(cmd *cobra.Command, args []string) error {
	var id string
	var remote bool
	var prov *core.Provider

	var err error

	id, remote, err = remoteResolveId(cmd, "/providers", args[0])
	if err != nil {
		return err
	}

	if remote {
		_, err = remoteDo(cmd, http.MethodPost, "/providers/"+id+"/activate")
		if err != nil {
			return err
		}

		fmt.Printf("provider %s is now active\n", id)

		return nil
	}

	prov, err = resolveProvider(args[0])
	if err != nil {
		return err
	}

	err = core.ProviderActivate(prov.Id)
	if err != nil {
		return err
	}

	fmt.Printf("provider %s is now active\n", prov.Id)

	return nil
}
