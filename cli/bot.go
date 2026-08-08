package main

import (
	"fmt"

	"github.com/devproje/mininaru/core"
	"github.com/spf13/cobra"
)

var (
	botNameRef  string
	botKindRef  string
	botTokenRef string
	botAgentRef string
	botGuildRef string
)

var botConfig *cobra.Command = &cobra.Command{
	Use:   "bot",
	Short: "manage chat bot front ends",
}

var botAdd *cobra.Command = &cobra.Command{
	Use:   "add",
	Short: "add a new bot",
	RunE:  botAddExecute,
}

var botList *cobra.Command = &cobra.Command{
	Use:   "list",
	Short: "list bots",
	RunE:  botListExecute,
}

var botUpdate *cobra.Command = &cobra.Command{
	Use:   "update <id or name>",
	Short: "update a bot",
	Args:  cobra.ExactArgs(1),
	RunE:  botUpdateExecute,
}

var botRemove *cobra.Command = &cobra.Command{
	Use:   "remove <id or name>",
	Short: "remove a bot",
	Args:  cobra.ExactArgs(1),
	RunE:  botRemoveExecute,
}

var botEnable *cobra.Command = &cobra.Command{
	Use:   "enable <id or name>",
	Short: "start this bot on the next serve",
	Args:  cobra.ExactArgs(1),
	RunE:  botEnableExecute,
}

var botDisable *cobra.Command = &cobra.Command{
	Use:   "disable <id or name>",
	Short: "keep this bot configured but stop starting it",
	Args:  cobra.ExactArgs(1),
	RunE:  botDisableExecute,
}

var botPair *cobra.Command = &cobra.Command{
	Use:   "pair <id or name>",
	Short: "create a one-time Discord admin pairing code",
	Args:  cobra.ExactArgs(1),
	RunE:  botPairExecute,
}

func botPairExecute(cmd *cobra.Command, args []string) error {
	var target *core.Bot
	var code string

	var err error

	target, err = core.BotFind(args[0])
	if err != nil {
		return err
	}
	code, err = core.DiscordPairCreate(target.Id)
	if err != nil {
		return err
	}
	fmt.Printf("Run /pair code:%s in Discord within 10 minutes to pair with %s\n", code, target.Name)
	return nil
}

func agentNames() []string {
	var cur *core.NaruAgent
	var names []string

	for _, cur = range core.AgentAll() {
		names = append(names, cur.Name)
	}

	return names
}

func botAddAsk() error {
	var err error

	if botNameRef == "" {
		botNameRef, err = askRequired("bot name")
		if err != nil {
			return err
		}
	}

	if botTokenRef == "" {
		botTokenRef, err = askSecret("bot token", false)
		if err != nil {
			return err
		}
	}

	if botAgentRef == "" && len(core.AgentAll()) > 1 {
		botAgentRef, err = askChoice("agent new channels talk to, empty for the global agent", agentNames(), "")
		if err != nil {
			return err
		}
	}

	if botGuildRef == "" {
		botGuildRef, err = askText("guild id for instant slash commands, empty for global", "")
		if err != nil {
			return err
		}
	}

	return nil
}

func botAddExecute(cmd *cobra.Command, args []string) error {
	var created *core.Bot

	var err error

	if askInteractive() {
		err = botAddAsk()
		if err != nil {
			return err
		}
	}

	created, err = core.BotCreate(core.Bot{
		Name:    botNameRef,
		Kind:    botKindRef,
		Token:   botTokenRef,
		Agent:   botAgentRef,
		GuildId: botGuildRef,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\t%s\n", created.Id, created.Name, created.Kind)

	return nil
}

func botAgentLabel(name string) string {
	if name == "" {
		return "[global]"
	}

	return name
}

func botState(enabled bool) string {
	if enabled {
		return "enabled"
	}

	return "disabled"
}

func botListExecute(cmd *cobra.Command, args []string) error {
	var cur *core.Bot

	for _, cur = range core.Bots {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n",
			cur.Id, cur.Name, cur.Kind, maskSecret(cur.Token), botAgentLabel(cur.Agent), botState(cur.Enabled))
	}

	return nil
}

func botUpdateAsk(current *core.Bot) error {
	var err error

	fmt.Fprintf(askOut, "updating bot %s, press enter to keep a value\n", current.Name)

	botNameRef, err = askText("bot name", current.Name)
	if err != nil {
		return err
	}

	botTokenRef, err = askSecret("bot token", true)
	if err != nil {
		return err
	}

	botAgentRef, err = askText("agent, empty for the global agent", current.Agent)
	if err != nil {
		return err
	}

	botGuildRef, err = askText("guild id", current.GuildId)

	return err
}

func botUpdateTouched(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("name") || cmd.Flags().Changed("token") ||
		cmd.Flags().Changed("agent") || cmd.Flags().Changed("guild")
}

func botUpdateExecute(cmd *cobra.Command, args []string) error {
	var name, token, agent, guildId *string
	var current *core.Bot

	var err error

	if !botUpdateTouched(cmd) && askInteractive() {
		current, err = core.BotFind(args[0])
		if err != nil {
			return err
		}

		err = botUpdateAsk(current)
		if err != nil {
			return err
		}

		if botTokenRef != "" {
			token = &botTokenRef
		}

		return core.BotUpdateFields(current.Id, &botNameRef, token, &botAgentRef, &botGuildRef, nil)
	}

	if cmd.Flags().Changed("name") {
		name = &botNameRef
	}
	if cmd.Flags().Changed("token") {
		token = &botTokenRef
	}
	if cmd.Flags().Changed("agent") {
		agent = &botAgentRef
	}
	if cmd.Flags().Changed("guild") {
		guildId = &botGuildRef
	}

	return core.BotUpdateFields(args[0], name, token, agent, guildId, nil)
}

func botRemoveExecute(cmd *cobra.Command, args []string) error {
	return core.BotDelete(args[0])
}

func botToggle(ref string, enabled bool) error {
	return core.BotUpdateFields(ref, nil, nil, nil, nil, &enabled)
}

func botEnableExecute(cmd *cobra.Command, args []string) error {
	return botToggle(args[0], true)
}

func botDisableExecute(cmd *cobra.Command, args []string) error {
	return botToggle(args[0], false)
}

func init() {
	botAdd.Flags().StringVarP(&botNameRef, "name", "n", "", "bot name")
	botAdd.Flags().StringVarP(&botKindRef, "kind", "k", core.BotDiscord, "bot kind, currently only discord")
	botAdd.Flags().StringVarP(&botTokenRef, "token", "t", "", "bot token")
	botAdd.Flags().StringVarP(&botAgentRef, "agent", "a", "", "agent new channels talk to, defaults to the global agent")
	botAdd.Flags().StringVarP(&botGuildRef, "guild", "g", "", "register slash commands to this guild only, which applies them instantly")

	botUpdate.Flags().StringVarP(&botNameRef, "name", "n", "", "bot name")
	botUpdate.Flags().StringVarP(&botTokenRef, "token", "t", "", "bot token")
	botUpdate.Flags().StringVarP(&botAgentRef, "agent", "a", "", "agent new channels talk to, pass an empty value to fall back to the global agent")
	botUpdate.Flags().StringVarP(&botGuildRef, "guild", "g", "", "guild id for slash command registration")

	botConfig.AddCommand(botAdd, botList, botUpdate, botRemove, botEnable, botDisable, botPair)
}
