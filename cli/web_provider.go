// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var webProviderKinds = []string{modules.WebBackendBrave, modules.WebBackendTavily, modules.WebBackendOllama}

var webProviderCmd *cobra.Command = &cobra.Command{
	Use:   "webprovider",
	Short: "manage web_search / web_fetch backends",
}

var webProviderAddCmd *cobra.Command = &cobra.Command{
	Use:   "add <name>",
	Short: "register a new web provider",

	Args: cobra.ExactArgs(1),
	RunE: webProviderAddExecute,
}

var webProviderListCmd *cobra.Command = &cobra.Command{
	Use:   "list",
	Short: "list registered web providers",
	RunE:  webProviderListExecute,
}

var webProviderShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <id-or-name>",
	Short: "show a web provider",

	Args: cobra.ExactArgs(1),
	RunE: webProviderShowExecute,
}

var webProviderSetCmd *cobra.Command = &cobra.Command{
	Use:   "set <id-or-name>",
	Short: "update a web provider",

	Args: cobra.ExactArgs(1),
	RunE: webProviderSetExecute,
}

var webProviderRemoveCmd *cobra.Command = &cobra.Command{
	Use:     "remove <id-or-name>",
	Aliases: []string{"rm", "delete"},
	Short:   "remove a web provider",

	Args: cobra.ExactArgs(1),
	RunE: webProviderRemoveExecute,
}

var webProviderPrimaryCmd *cobra.Command = &cobra.Command{
	Use:   "primary <id-or-name>",
	Short: "make a web provider the selected one",

	Args: cobra.ExactArgs(1),
	RunE: webProviderPrimaryExecute,
}

var (
	webProviderAddKindRef    string
	webProviderAddApiKeyRef  string
	webProviderAddBaseUrlRef string

	webProviderSetNameRef    string
	webProviderSetKindRef    string
	webProviderSetApiKeyRef  string
	webProviderSetBaseUrlRef string
)

func init() {
	webProviderAddCmd.Flags().StringVar(&webProviderAddKindRef, "kind", "", "backend kind: brave, tavily, or ollama")
	webProviderAddCmd.Flags().StringVar(&webProviderAddApiKeyRef, "api-key", "", "API key for the backend")
	webProviderAddCmd.Flags().StringVar(&webProviderAddBaseUrlRef, "base-url", "", "base URL override for the backend")

	webProviderSetCmd.Flags().StringVar(&webProviderSetNameRef, "name", "", "new name for the web provider")
	webProviderSetCmd.Flags().StringVar(&webProviderSetKindRef, "kind", "", "new backend kind: brave, tavily, or ollama")
	webProviderSetCmd.Flags().StringVar(&webProviderSetApiKeyRef, "api-key", "", "new API key for the backend")
	webProviderSetCmd.Flags().StringVar(&webProviderSetBaseUrlRef, "base-url", "", "new base URL for the backend")

	webProviderCmd.AddCommand(webProviderAddCmd, webProviderListCmd, webProviderShowCmd, webProviderSetCmd, webProviderRemoveCmd, webProviderPrimaryCmd)
}

func validWebProviderKind(kind string) bool {
	var candidate string

	for _, candidate = range webProviderKinds {
		if candidate == kind {
			return true
		}
	}

	return false
}

func resolveWebProvider(idOrName string) (*core.WebProvider, error) {
	var prov *core.WebProvider

	var err error

	prov, err = core.WebProviderRead(idOrName)
	if err == nil {
		return prov, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	prov, err = core.WebProviderByName(idOrName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("web provider %q not found", idOrName)
		}

		return nil, err
	}

	return prov, nil
}

func printWebProvider(prov *core.WebProvider) {
	var selected string

	selected = ""
	if prov.Selected {
		selected = "  (selected)"
	}

	fmt.Printf("%s  %s%s\n", prov.Id, prov.Name, selected)
	fmt.Printf("  kind      %s\n", prov.Kind)
	fmt.Printf("  base_url  %s\n", prov.BaseUrl)
	fmt.Printf("  api_key   %s\n", maskSecret(prov.ApiKey))
}

func webProviderAddExecute(cmd *cobra.Command, args []string) error {
	var prov core.WebProvider

	var err error

	if !validWebProviderKind(webProviderAddKindRef) {
		return fmt.Errorf("--kind must be one of: brave, tavily, ollama")
	}

	prov = core.WebProvider{
		Id:      uuid.NewString(),
		Name:    args[0],
		Kind:    webProviderAddKindRef,
		ApiKey:  webProviderAddApiKeyRef,
		BaseUrl: webProviderAddBaseUrlRef,
	}

	err = core.WebProviderCreate(&prov)
	if err != nil {
		return err
	}

	fmt.Printf("web provider %s created (%s)\n", prov.Name, prov.Id)

	return nil
}

func webProviderListExecute(cmd *cobra.Command, args []string) error {
	var list []*core.WebProvider
	var prov *core.WebProvider

	var err error

	list, err = core.WebProviderList()
	if err != nil {
		return err
	}

	if len(list) == 0 {
		fmt.Println("no web providers registered")
		return nil
	}

	for _, prov = range list {
		printWebProvider(prov)
	}

	return nil
}

func webProviderShowExecute(cmd *cobra.Command, args []string) error {
	var prov *core.WebProvider

	var err error

	prov, err = resolveWebProvider(args[0])
	if err != nil {
		return err
	}

	printWebProvider(prov)

	return nil
}

func webProviderSetExecute(cmd *cobra.Command, args []string) error {
	var prov *core.WebProvider

	var err error

	prov, err = resolveWebProvider(args[0])
	if err != nil {
		return err
	}

	if webProviderSetKindRef != "" && !validWebProviderKind(webProviderSetKindRef) {
		return fmt.Errorf("--kind must be one of: brave, tavily, ollama")
	}

	err = core.WebProviderUpdate(prov.Id, &core.WebProvider{
		Name:    webProviderSetNameRef,
		Kind:    webProviderSetKindRef,
		ApiKey:  webProviderSetApiKeyRef,
		BaseUrl: webProviderSetBaseUrlRef,
	})
	if err != nil {
		return err
	}

	fmt.Printf("web provider %s updated\n", prov.Id)

	return nil
}

func webProviderRemoveExecute(cmd *cobra.Command, args []string) error {
	var prov *core.WebProvider

	var err error

	prov, err = resolveWebProvider(args[0])
	if err != nil {
		return err
	}

	err = core.WebProviderDelete(prov.Id)
	if err != nil {
		return err
	}

	fmt.Printf("web provider %s removed\n", prov.Id)

	return nil
}

func webProviderPrimaryExecute(cmd *cobra.Command, args []string) error {
	var prov *core.WebProvider

	var err error

	prov, err = resolveWebProvider(args[0])
	if err != nil {
		return err
	}

	err = core.WebProviderSelect(prov.Id)
	if err != nil {
		return err
	}

	fmt.Printf("web provider %s is now primary\n", prov.Id)

	return nil
}
