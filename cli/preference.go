package main

import (
	"fmt"

	"github.com/devproje/mininaru/core"
	"github.com/spf13/cobra"
)

var (
	providerNameRef    string
	providerApiKeyRef  string
	providerBaseURLRef string

	agentNameRef     string
	agentRoleRef     string
	agentSoulRef     string
	agentModelRef    string
	agentProviderRef string

	sessionAgentIdRef string
	sessionNameRef    string
)

var provider *cobra.Command = &cobra.Command{
	Use:   "provider",
	Short: "manage LLM providers",
	Long: `Manage the OpenAI compatible endpoints agents talk to.

A provider bundles a base URL and an API key. Agents pick a provider when they
are created, and new agents fall back to the default one.`,
	Example: `  mininaru provider add --name openai --base-url https://api.openai.com/v1
  mininaru provider list
  mininaru provider default openai`,
}

var providerAdd *cobra.Command = &cobra.Command{
	Use:   "add",
	Short: "add a new provider",
	Long: `Add a provider.

On a terminal any value you leave out is prompted for. Without a terminal every
value must come from a flag.`,
	Example: `  mininaru provider add --name openai --base-url https://api.openai.com/v1 --api-key sk-...`,
	Args:    usageArgs(cobra.NoArgs),
	RunE:    providerAddExecute,
}

var providerList *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list providers",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    providerListExecute,
}

var providerUpdate *cobra.Command = &cobra.Command{
	Use:   "update <id>",
	Short: "update a provider",
	Long: `Update a provider by id or name.

Only the fields you pass as flags change. On a terminal, passing no flag at all
walks through every field with the current value as the default.`,
	Example: `  mininaru provider update openai --api-key sk-...`,
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    providerUpdateExecute,
}

var providerRemove *cobra.Command = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm"},
	Short:   "remove a provider",
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    providerRemoveExecute,
}

var providerDefault *cobra.Command = &cobra.Command{
	Use:     "default [id or name]",
	Short:   "show or set the provider new agents use by default",
	Example: `  mininaru provider default openai`,
	Args:    usageArgs(cobra.MaximumNArgs(1)),
	RunE:    providerDefaultExecute,
}

var agent *cobra.Command = &cobra.Command{
	Use:   "agent",
	Short: "manage agents",
	Long: `Manage the agents you can chat with.

An agent pairs a provider and model with a role and a soul, the two prompts that
shape how it answers. The global agent is the one the chat client opens by
default and the one bots use when they name none.`,
	Example: `  mininaru agent add --name reviewer --model gpt-4o
  mininaru agent default reviewer`,
}

var agentAdd *cobra.Command = &cobra.Command{
	Use:   "add",
	Short: "add a new agent",
	Long: `Add an agent.

On a terminal any value you leave out is prompted for. The agent uses the
default provider unless --provider names another one.`,
	Example: `  mininaru agent add --name reviewer --model gpt-4o --role "code reviewer"`,
	Args:    usageArgs(cobra.NoArgs),
	RunE:    agentAddExecute,
}

var agentList *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list agents",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    agentListExecute,
}

var agentUpdate *cobra.Command = &cobra.Command{
	Use:   "update <id>",
	Short: "update an agent",
	Long: `Update an agent by id or name.

Only the fields you pass as flags change. On a terminal, passing no flag at all
walks through every field with the current value as the default.`,
	Example: `  mininaru agent update reviewer --model gpt-4o-mini`,
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    agentUpdateExecute,
}

var agentRemove *cobra.Command = &cobra.Command{
	Use:     "remove <id or name>",
	Aliases: []string{"rm"},
	Short:   "remove an agent and its sessions",
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    agentRemoveExecute,
}

var agentDefault *cobra.Command = &cobra.Command{
	Use:     "default [id or name]",
	Short:   "show or set the global agent the client uses by default",
	Example: `  mininaru agent default reviewer`,
	Args:    usageArgs(cobra.MaximumNArgs(1)),
	RunE:    agentDefaultExecute,
}

var session *cobra.Command = &cobra.Command{
	Use:   "session",
	Short: "manage chat sessions",
	Long: `Manage stored conversations.

Every session belongs to one agent, so these commands act on the global agent
unless --agent names another one. Resume a session with ` + "`mininaru --session <id>`" + `.`,
	Example: `  mininaru session list
  mininaru session rename 3f2a --name "release notes"`,
}

var sessionList *cobra.Command = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list sessions",
	Args:    usageArgs(cobra.NoArgs),
	RunE:    sessionListExecute,
}

var sessionRemove *cobra.Command = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm"},
	Short:   "remove a session",
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    sessionRemoveExecute,
}

var sessionRename *cobra.Command = &cobra.Command{
	Use:   "rename <id>",
	Short: "rename a chat session",
	Long: `Rename a session.

On a terminal the new name is prompted for when --name is omitted.`,
	Example: `  mininaru session rename 3f2a --name "release notes"`,
	Args:    usageArgs(cobra.ExactArgs(1)),
	RunE:    sessionRenameExecute,
}

func providerAddAsk() error {
	var err error

	if providerNameRef == "" {
		providerNameRef, err = askRequired("provider name")
		if err != nil {
			return err
		}
	}

	if providerBaseURLRef == "" {
		providerBaseURLRef, err = askRequired("base url")
		if err != nil {
			return err
		}
	}

	if providerApiKeyRef == "" {
		providerApiKeyRef, err = askSecret("api key", true)
		if err != nil {
			return err
		}
	}

	return nil
}

func providerAddExecute(cmd *cobra.Command, args []string) error {
	var payload core.Provider

	var err error

	if askInteractive() {
		err = providerAddAsk()
		if err != nil {
			return err
		}
	}

	if providerNameRef == "" {
		return usageErrorf("provider name is required, pass --name")
	}

	payload = core.Provider{
		Name:    providerNameRef,
		ApiKey:  providerApiKeyRef,
		BaseURL: providerBaseURLRef,
	}

	core.ProviderCreate(payload)

	return core.ProviderSave()
}

func providerListExecute(cmd *cobra.Command, args []string) error {
	var rows *uiRows
	var cur *core.Provider
	var mark string

	if len(core.Providers) == 0 {
		uiEmpty("no providers yet, add one with `mininaru provider add`")

		return nil
	}

	rows = uiTable("ID", "NAME", "BASE URL", "API KEY", "")

	for _, cur = range core.Providers {
		mark = ""
		if cur == core.DefaultProvider {
			mark = "[default]"
		}

		rows.row(cur.Id, cur.Name, cur.BaseURL, maskSecret(cur.ApiKey), mark)
	}

	rows.flush()

	return nil
}

func providerDefaultExecute(cmd *cobra.Command, args []string) error {
	var rows *uiRows

	if len(args) == 0 {
		if core.DefaultProvider == nil {
			return configErrorf("no provider configured, add one with `mininaru provider add` first")
		}

		rows = uiTable("ID", "NAME")
		rows.row(core.DefaultProvider.Id, core.DefaultProvider.Name)
		rows.flush()

		return nil
	}

	return core.ProviderDefault(args[0])
}

func providerUpdateAsk(current *core.Provider) error {
	var err error

	fmt.Fprintf(askOut, "updating provider %s, press enter to keep a value\n", current.Name)

	providerNameRef, err = askText("provider name", current.Name)
	if err != nil {
		return err
	}

	providerBaseURLRef, err = askText("base url", current.BaseURL)
	if err != nil {
		return err
	}

	providerApiKeyRef, err = askSecret("api key", true)
	if err != nil {
		return err
	}

	return nil
}

func providerUpdateExecute(cmd *cobra.Command, args []string) error {
	var touched bool
	var current *core.Provider
	var name, apiKey, baseURL *string

	var err error

	touched = cmd.Flags().Changed("name") || cmd.Flags().Changed("api-key") || cmd.Flags().Changed("base-url")

	if !touched && askInteractive() {
		current, err = core.ProviderFind(args[0])
		if err != nil {
			return err
		}

		err = providerUpdateAsk(current)
		if err != nil {
			return err
		}

		if providerApiKeyRef != "" {
			apiKey = &providerApiKeyRef
		}

		return core.ProviderUpdateFields(current.Id, &providerNameRef, apiKey, &providerBaseURLRef)
	}

	if cmd.Flags().Changed("name") {
		name = &providerNameRef
	}
	if cmd.Flags().Changed("api-key") {
		apiKey = &providerApiKeyRef
	}
	if cmd.Flags().Changed("base-url") {
		baseURL = &providerBaseURLRef
	}

	return core.ProviderUpdateFields(args[0], name, apiKey, baseURL)
}

func providerRemoveExecute(cmd *cobra.Command, args []string) error {
	return core.ProviderDelete(args[0])
}

func resolveProvider() (*core.Provider, error) {
	if agentProviderRef != "" {
		return core.ProviderFind(agentProviderRef)
	}

	if core.DefaultProvider == nil {
		return nil, configErrorf("no provider configured, add one with `mininaru provider add` first")
	}

	return core.DefaultProvider, nil
}

func providerNames() []string {
	var cur *core.Provider
	var names []string

	for _, cur = range core.Providers {
		names = append(names, cur.Name)
	}

	return names
}

func agentAddAsk() error {
	var fallback string

	var err error

	if agentNameRef == "" {
		agentNameRef, err = askRequired("agent name")
		if err != nil {
			return err
		}
	}

	if agentModelRef == "" {
		agentModelRef, err = askRequired("model")
		if err != nil {
			return err
		}
	}

	if agentRoleRef == "" {
		agentRoleRef, err = askText("role", "")
		if err != nil {
			return err
		}
	}

	if agentSoulRef == "" {
		agentSoulRef, err = askText("soul", "")
		if err != nil {
			return err
		}
	}

	if agentProviderRef != "" || len(core.Providers) < 2 {
		return nil
	}

	if core.DefaultProvider != nil {
		fallback = core.DefaultProvider.Name
	}

	agentProviderRef, err = askChoice("provider", providerNames(), fallback)

	return err
}

func agentAddExecute(cmd *cobra.Command, args []string) error {
	var prov *core.Provider
	var newAgent *core.NaruAgent

	var err error

	if askInteractive() {
		err = agentAddAsk()
		if err != nil {
			return err
		}
	}

	if agentNameRef == "" {
		return fmt.Errorf("agent name is required, pass --name")
	}
	if agentModelRef == "" {
		return fmt.Errorf("agent model is required, pass --model")
	}

	prov, err = resolveProvider()
	if err != nil {
		return err
	}

	if core.Global == nil {
		newAgent = core.AgentNew(agentNameRef, agentRoleRef, agentSoulRef, agentModelRef, prov)
		if newAgent == nil {
			return fmt.Errorf("failed to create agent")
		}

		core.Global = newAgent

		return core.AgentSave()
	}

	return core.AgentCreate(agentNameRef, agentRoleRef, agentSoulRef, agentModelRef, prov)
}

func providerLabel(id string) string {
	var prov *core.Provider

	var err error

	prov, err = core.ProviderFind(id)
	if err != nil {
		return "-"
	}

	return prov.Name
}

func agentListExecute(cmd *cobra.Command, args []string) error {
	var rows *uiRows
	var all []*core.NaruAgent
	var cur *core.NaruAgent
	var mark string

	all = core.AgentAll()
	if len(all) == 0 {
		uiEmpty("no agents yet, add one with `mininaru agent add`")

		return nil
	}

	rows = uiTable("ID", "NAME", "MODEL", "PROVIDER", "")

	for _, cur = range all {
		mark = ""
		if core.Global != nil && cur.Id == core.Global.Id {
			mark = "[global]"
		}

		rows.row(cur.Id, cur.Name, cur.Model, providerLabel(cur.ProviderId), mark)
	}

	rows.flush()

	return nil
}

func agentUpdateAsk(current *core.NaruAgent) error {
	var err error

	fmt.Fprintf(askOut, "updating agent %s, press enter to keep a value\n", current.Name)

	agentNameRef, err = askText("agent name", current.Name)
	if err != nil {
		return err
	}

	agentModelRef, err = askText("model", current.Model)
	if err != nil {
		return err
	}

	agentRoleRef, err = askText("role", current.Role)
	if err != nil {
		return err
	}

	agentSoulRef, err = askText("soul", current.Soul)
	if err != nil {
		return err
	}

	if len(core.Providers) < 2 {
		return nil
	}

	agentProviderRef, err = askChoice("provider", providerNames(), providerLabel(current.ProviderId))

	return err
}

func agentApplyUpdate(ref string, name, role, soul, model, providerId *string) error {
	var err error

	if core.Global == nil || core.Global.Id != ref {
		return core.AgentUpdateFields(ref, name, role, soul, model, providerId)
	}

	if name != nil {
		core.Global.Name = *name
	}

	if role != nil {
		core.Global.Role = *role
	}

	if soul != nil {
		core.Global.Soul = *soul
	}

	if model != nil {
		core.Global.Model = *model
	}

	if providerId != nil {
		core.Global.ProviderId = *providerId

		err = core.AgentRefreshClient(core.Global)
		if err != nil {
			return err
		}
	}

	return core.AgentSave()
}

func agentUpdateTouched(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("name") || cmd.Flags().Changed("role") ||
		cmd.Flags().Changed("soul") || cmd.Flags().Changed("model") || cmd.Flags().Changed("provider")
}

func agentUpdateExecute(cmd *cobra.Command, args []string) error {
	var name, role, soul, model *string
	var prov *core.Provider
	var providerId *string
	var current *core.NaruAgent

	var err error

	if !agentUpdateTouched(cmd) && askInteractive() {
		current, err = core.AgentByName(args[0])
		if err != nil {
			return err
		}

		err = agentUpdateAsk(current)
		if err != nil {
			return err
		}

		name, role, soul, model = &agentNameRef, &agentRoleRef, &agentSoulRef, &agentModelRef

		if agentProviderRef != "" {
			prov, err = core.ProviderFind(agentProviderRef)
			if err != nil {
				return err
			}

			providerId = &prov.Id
		}

		return agentApplyUpdate(current.Id, name, role, soul, model, providerId)
	}

	if cmd.Flags().Changed("name") {
		name = &agentNameRef
	}
	if cmd.Flags().Changed("role") {
		role = &agentRoleRef
	}
	if cmd.Flags().Changed("soul") {
		soul = &agentSoulRef
	}
	if cmd.Flags().Changed("model") {
		model = &agentModelRef
	}

	if agentProviderRef != "" {
		prov, err = core.ProviderFind(agentProviderRef)
		if err != nil {
			return err
		}

		providerId = &prov.Id
	}

	return agentApplyUpdate(args[0], name, role, soul, model, providerId)
}

func agentRemoveExecute(cmd *cobra.Command, args []string) error {
	return core.AgentDelete(args[0])
}

func agentDefaultExecute(cmd *cobra.Command, args []string) error {
	var rows *uiRows

	var err error

	if len(args) == 0 {
		if core.Global == nil {
			return configErrorf("no global agent configured, set one with `mininaru agent default <name>`")
		}

		rows = uiTable("ID", "NAME")
		rows.row(core.Global.Id, core.Global.Name)
		rows.flush()

		return nil
	}

	err = core.AgentDefault(args[0])
	if err != nil {
		return err
	}

	rows = uiTable("ID", "NAME")
	rows.row(core.Global.Id, core.Global.Name)
	rows.flush()

	return nil
}

func sessionAgent() (*core.NaruAgent, error) {
	if sessionAgentIdRef != "" {
		return core.AgentByName(sessionAgentIdRef)
	}

	if core.Global == nil {
		return nil, configErrorf("no agent configured, run `mininaru setup` or add one with `mininaru agent add`")
	}

	return core.Global, nil
}

func sessionListExecute(cmd *cobra.Command, args []string) error {
	var target *core.NaruAgent
	var sessions []*core.Session
	var cur *core.Session
	var rows *uiRows

	var err error

	target, err = sessionAgent()
	if err != nil {
		return err
	}

	sessions, err = core.SessionList(target.Id)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		uiEmpty("no sessions for %s yet, run `mininaru` to start one", target.Name)

		return nil
	}

	rows = uiTable("ID", "NAME")

	for _, cur = range sessions {
		rows.row(cur.Id, cur.Name)
	}

	rows.flush()

	return nil
}

func sessionRemoveExecute(cmd *cobra.Command, args []string) error {
	var target *core.NaruAgent
	var owned *core.Session

	var err error

	target, err = sessionAgent()
	if err != nil {
		return err
	}

	owned, err = core.SessionFind(args[0])
	if err != nil {
		return err
	}

	if owned.AgentId != target.Id {
		return fmt.Errorf("session %s does not belong to %s", owned.Id, target.Name)
	}

	return core.SessionDelete(args[0])
}

func sessionRenameExecute(cmd *cobra.Command, args []string) error {
	var err error

	if sessionNameRef == "" && askInteractive() {
		sessionNameRef, err = askRequired("new session name")
		if err != nil {
			return err
		}
	}

	if sessionNameRef == "" {
		return usageErrorf("session name is required, pass --name")
	}

	return core.SessionUpdate(args[0], sessionNameRef)
}

func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}

	if len(secret) <= 4 {
		return "****"
	}

	return secret[:4] + "****"
}

func init() {
	providerAdd.Flags().StringVarP(&providerNameRef, "name", "n", "", "provider name")
	providerAdd.Flags().StringVarP(&providerApiKeyRef, "api-key", "k", "", "provider api key")
	providerAdd.Flags().StringVarP(&providerBaseURLRef, "base-url", "b", "", "provider base url")

	providerUpdate.Flags().StringVarP(&providerNameRef, "name", "n", "", "provider name")
	providerUpdate.Flags().StringVarP(&providerApiKeyRef, "api-key", "k", "", "provider api key")
	providerUpdate.Flags().StringVarP(&providerBaseURLRef, "base-url", "b", "", "provider base url")

	provider.AddCommand(providerAdd, providerList, providerUpdate, providerRemove, providerDefault)

	agentAdd.Flags().StringVarP(&agentNameRef, "name", "n", "", "agent name")
	agentAdd.Flags().StringVarP(&agentRoleRef, "role", "r", "", "agent role")
	agentAdd.Flags().StringVarP(&agentSoulRef, "soul", "s", "", "agent soul prompt")
	agentAdd.Flags().StringVarP(&agentModelRef, "model", "m", "", "agent model")
	agentAdd.Flags().StringVarP(&agentProviderRef, "provider", "p", "", "provider id or name, defaults to the default provider")

	agentUpdate.Flags().StringVarP(&agentNameRef, "name", "n", "", "agent name")
	agentUpdate.Flags().StringVarP(&agentRoleRef, "role", "r", "", "agent role")
	agentUpdate.Flags().StringVarP(&agentSoulRef, "soul", "s", "", "agent soul prompt")
	agentUpdate.Flags().StringVarP(&agentModelRef, "model", "m", "", "agent model")
	agentUpdate.Flags().StringVarP(&agentProviderRef, "provider", "p", "", "provider id or name")

	agent.AddCommand(agentAdd, agentList, agentUpdate, agentRemove, agentDefault)

	sessionList.Flags().StringVarP(&sessionAgentIdRef, "agent", "a", "", "agent name or id, defaults to the global agent")
	sessionRemove.Flags().StringVarP(&sessionAgentIdRef, "agent", "a", "", "agent name or id that owns the session, defaults to the global agent")
	sessionRename.Flags().StringVarP(&sessionNameRef, "name", "n", "", "new session name")

	session.AddCommand(sessionList, sessionRemove, sessionRename)
}
