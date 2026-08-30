// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/devproje/mininaru/modules/client"
	"github.com/devproje/mininaru/util"
	"github.com/spf13/cobra"
)

type gatewayEntry struct {
	Url    string `json:"url"`
	ApiKey string `json:"api_key,omitempty"`
}

type gatewayStore map[string]gatewayEntry

const gatewayStorePath = "gateways.json"

var gatewayNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

var (
	gatewayRef string

	gatewayAddApiKeyRef string
	gatewaySetUrlRef    string
	gatewaySetApiKeyRef string
)

var gatewayCmd *cobra.Command = &cobra.Command{
	Use:   "gateway",
	Short: "manage named remote mininaru endpoints",
}

var gatewayAddCmd *cobra.Command = &cobra.Command{
	Use:   "add <name> <ws-url>",
	Short: "register a remote endpoint",

	Args: cobra.ExactArgs(2),
	RunE: gatewayAddExecute,
}

var gatewayListCmd *cobra.Command = &cobra.Command{
	Use:   "list",
	Short: "list registered endpoints",
	RunE:  gatewayListExecute,
}

var gatewayShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <name>",
	Short: "show an endpoint",

	Args: cobra.ExactArgs(1),
	RunE: gatewayShowExecute,
}

var gatewaySetCmd *cobra.Command = &cobra.Command{
	Use:   "set <name>",
	Short: "update an endpoint",

	Args: cobra.ExactArgs(1),
	RunE: gatewaySetExecute,
}

var gatewayRemoveCmd *cobra.Command = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete"},
	Short:   "remove an endpoint",

	Args: cobra.ExactArgs(1),
	RunE: gatewayRemoveExecute,
}

func init() {
	gatewayAddCmd.Flags().StringVar(&gatewayAddApiKeyRef, "api-key", "", "api key for the remote server")

	gatewaySetCmd.Flags().StringVar(&gatewaySetUrlRef, "url", "", "new websocket url")
	gatewaySetCmd.Flags().StringVar(&gatewaySetApiKeyRef, "api-key", "", "new api key")

	gatewayCmd.AddCommand(gatewayAddCmd, gatewayListCmd, gatewayShowCmd, gatewaySetCmd, gatewayRemoveCmd)
}

func loadGateways() (gatewayStore, error) {
	var store gatewayStore
	var buf []byte

	var err error

	store = gatewayStore{}

	buf, err = os.ReadFile(util.Path(gatewayStorePath))
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}

		return nil, err
	}

	err = json.Unmarshal(buf, &store)
	if err != nil {
		return nil, err
	}

	return store, nil
}

func saveGateways(store gatewayStore) error {
	var buf []byte

	var err error

	buf, err = json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(util.Path(gatewayStorePath), buf, 0600)
}

func resolveGateway() (*client.Gateway, error) {
	var store gatewayStore
	var entry gatewayEntry
	var ok bool

	var err error

	if gatewayRef == "" {
		return nil, nil
	}

	store, err = loadGateways()
	if err != nil {
		return nil, err
	}

	entry, ok = store[gatewayRef]
	if !ok {
		return nil, fmt.Errorf("gateway %q not registered — see 'mininaru gateway add'", gatewayRef)
	}

	return &client.Gateway{Name: gatewayRef, Url: entry.Url, ApiKey: entry.ApiKey}, nil
}

func applyGateway(cmd *cobra.Command) error {
	var gw *client.Gateway

	var err error

	gw, err = resolveGateway()
	if err != nil {
		return err
	}

	if gw == nil {
		return nil
	}

	if cmd.Flags().Changed("url") || cmd.Flags().Changed("api-key") {
		return fmt.Errorf("--gateway cannot be combined with --url or --api-key")
	}

	promptUrlRef = gw.Url
	promptApiKeyRef = gw.ApiKey

	return nil
}

func gatewayList() ([]client.Gateway, error) {
	var store gatewayStore
	var names []string
	var name string
	var list []client.Gateway

	var err error

	store, err = loadGateways()
	if err != nil {
		return nil, err
	}

	for name = range store {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name = range names {
		list = append(list, client.Gateway{Name: name, Url: store[name].Url, ApiKey: store[name].ApiKey})
	}

	return list, nil
}

func printGateway(name string, entry gatewayEntry) {
	fmt.Printf("%s\n", name)
	fmt.Printf("  url      %s\n", entry.Url)
	fmt.Printf("  api_key  %s\n", maskSecret(entry.ApiKey))
}

func gatewayAddExecute(cmd *cobra.Command, args []string) error {
	var store gatewayStore
	var ok bool

	var err error

	if !gatewayNamePattern.MatchString(args[0]) {
		return fmt.Errorf("invalid gateway name %q", args[0])
	}

	store, err = loadGateways()
	if err != nil {
		return err
	}

	_, ok = store[args[0]]
	if ok {
		return fmt.Errorf("gateway %q already exists", args[0])
	}

	store[args[0]] = gatewayEntry{Url: args[1], ApiKey: gatewayAddApiKeyRef}

	err = saveGateways(store)
	if err != nil {
		return err
	}

	fmt.Printf("gateway %s added\n", args[0])

	return nil
}

func gatewayListExecute(cmd *cobra.Command, args []string) error {
	var store gatewayStore
	var names []string
	var name string

	var err error

	store, err = loadGateways()
	if err != nil {
		return err
	}

	if len(store) == 0 {
		fmt.Println("no gateways registered")
		return nil
	}

	for name = range store {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name = range names {
		printGateway(name, store[name])
	}

	return nil
}

func gatewayShowExecute(cmd *cobra.Command, args []string) error {
	var store gatewayStore
	var entry gatewayEntry
	var ok bool

	var err error

	store, err = loadGateways()
	if err != nil {
		return err
	}

	entry, ok = store[args[0]]
	if !ok {
		return fmt.Errorf("gateway %q not found", args[0])
	}

	printGateway(args[0], entry)

	return nil
}

func gatewaySetExecute(cmd *cobra.Command, args []string) error {
	var store gatewayStore
	var entry gatewayEntry
	var ok bool

	var err error

	store, err = loadGateways()
	if err != nil {
		return err
	}

	entry, ok = store[args[0]]
	if !ok {
		return fmt.Errorf("gateway %q not found", args[0])
	}

	if gatewaySetUrlRef != "" {
		entry.Url = gatewaySetUrlRef
	}
	if cmd.Flags().Changed("api-key") {
		entry.ApiKey = gatewaySetApiKeyRef
	}

	store[args[0]] = entry

	err = saveGateways(store)
	if err != nil {
		return err
	}

	fmt.Printf("gateway %s updated\n", args[0])

	return nil
}

func gatewayRemoveExecute(cmd *cobra.Command, args []string) error {
	var store gatewayStore
	var ok bool

	var err error

	store, err = loadGateways()
	if err != nil {
		return err
	}

	if _, ok = store[args[0]]; !ok {
		return fmt.Errorf("gateway %q not found", args[0])
	}

	delete(store, args[0])

	err = saveGateways(store)
	if err != nil {
		return err
	}

	fmt.Printf("gateway %s removed\n", args[0])

	return nil
}
