// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/devproje/mininaru/core"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var agentCmd *cobra.Command = &cobra.Command{
	Use:   "agent",
	Short: "manage agents",
}

var agentAddCmd *cobra.Command = &cobra.Command{
	Use:   "add <name>",
	Short: "register a new agent",

	Args: cobra.ExactArgs(1),
	RunE: agentAddExecute,
}

var agentListCmd *cobra.Command = &cobra.Command{
	Use:   "list",
	Short: "list registered agents",
	RunE:  agentListExecute,
}

var agentShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <id-or-name>",
	Short: "show an agent",

	Args: cobra.ExactArgs(1),
	RunE: agentShowExecute,
}

var agentSetCmd *cobra.Command = &cobra.Command{
	Use:   "set <id-or-name>",
	Short: "update an agent",

	Args: cobra.ExactArgs(1),
	RunE: agentSetExecute,
}

var agentRemoveCmd *cobra.Command = &cobra.Command{
	Use:     "remove <id-or-name>",
	Aliases: []string{"rm", "delete"},
	Short:   "remove an agent",

	Args: cobra.ExactArgs(1),
	RunE: agentRemoveExecute,
}

var (
	agentAddModelRef      string
	agentAddSoulRef       string
	agentAddThinkingRef   string
	agentAddMaxContextRef uint64

	agentSetNameRef       string
	agentSetModelRef      string
	agentSetSoulRef       string
	agentSetThinkingRef   string
	agentSetMaxContextRef uint64
)

func init() {
	agentAddCmd.Flags().StringVar(&agentAddModelRef, "model", "", "model identifier the agent talks to (required)")
	agentAddCmd.Flags().StringVar(&agentAddSoulRef, "soul", "", "system prompt / persona for the agent")
	agentAddCmd.Flags().StringVar(&agentAddThinkingRef, "thinking", "", "reasoning effort: off, low, medium, high, max")
	agentAddCmd.Flags().Uint64Var(&agentAddMaxContextRef, "max-context", 0, "context window budget in tokens")
	agentAddCmd.MarkFlagRequired("model")

	agentSetCmd.Flags().StringVar(&agentSetNameRef, "name", "", "new name for the agent")
	agentSetCmd.Flags().StringVar(&agentSetModelRef, "model", "", "new model identifier")
	agentSetCmd.Flags().StringVar(&agentSetSoulRef, "soul", "", "new system prompt / persona")
	agentSetCmd.Flags().StringVar(&agentSetThinkingRef, "thinking", "", "new reasoning effort: off, low, medium, high, max")
	agentSetCmd.Flags().Uint64Var(&agentSetMaxContextRef, "max-context", 0, "new context window budget in tokens")

	agentCmd.AddCommand(agentAddCmd, agentListCmd, agentShowCmd, agentSetCmd, agentRemoveCmd)
}

func resolveAgent(idOrName string) (*core.Agent, error) {
	var agent *core.Agent

	var err error

	agent, err = core.AgentRead(idOrName)
	if err == nil {
		return agent, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	agent, err = core.AgentByName(idOrName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("agent %q not found", idOrName)
		}

		return nil, err
	}

	return agent, nil
}

func printAgent(agent *core.Agent) {
	fmt.Printf("%s  %s\n", agent.Id, agent.Name)
	fmt.Printf("  model         %s\n", agent.Model)
	fmt.Printf("  thinking      %s\n", agent.ThinkingLevel)
	fmt.Printf("  max_context   %d\n", agent.MaxContext)

	if agent.Soul != "" {
		fmt.Printf("  soul          %s\n", agent.Soul)
	}
}

func agentAddExecute(cmd *cobra.Command, args []string) error {
	var agent core.Agent

	var err error

	agent = core.Agent{
		Id:            uuid.NewString(),
		Name:          args[0],
		Model:         agentAddModelRef,
		Soul:          agentAddSoulRef,
		ThinkingLevel: agentAddThinkingRef,
		MaxContext:    agentAddMaxContextRef,
	}

	err = core.AgentCreate(&agent)
	if err != nil {
		return err
	}

	fmt.Printf("agent %s created (%s)\n", agent.Name, agent.Id)

	return nil
}

func agentListExecute(cmd *cobra.Command, args []string) error {
	var remote bool
	var list []*core.Agent
	var agent *core.Agent

	var err error

	remote, err = remoteGet(cmd, "/agents", &list)
	if err != nil {
		return err
	}

	if !remote {
		list, err = core.AgentList()
		if err != nil {
			return err
		}
	}

	if len(list) == 0 {
		fmt.Println("no agents registered")
		return nil
	}

	for _, agent = range list {
		printAgent(agent)
	}

	return nil
}

func agentShowExecute(cmd *cobra.Command, args []string) error {
	var remote bool
	var list []*core.Agent
	var item *core.Agent
	var agent *core.Agent

	var err error

	remote, err = remoteGet(cmd, "/agents", &list)
	if err != nil {
		return err
	}

	if !remote {
		agent, err = resolveAgent(args[0])
		if err != nil {
			return err
		}

		printAgent(agent)

		return nil
	}

	for _, item = range list {
		if item.Id == args[0] || item.Name == args[0] {
			printAgent(item)

			return nil
		}
	}

	return fmt.Errorf("agent %q not found", args[0])
}

func agentSetExecute(cmd *cobra.Command, args []string) error {
	var agent *core.Agent

	var err error

	agent, err = resolveAgent(args[0])
	if err != nil {
		return err
	}

	err = core.AgentUpdate(agent.Id, &core.Agent{
		Name:          agentSetNameRef,
		Model:         agentSetModelRef,
		Soul:          agentSetSoulRef,
		ThinkingLevel: agentSetThinkingRef,
		MaxContext:    agentSetMaxContextRef,
	})
	if err != nil {
		return err
	}

	fmt.Printf("agent %s updated\n", agent.Id)

	return nil
}

func agentRemoveExecute(cmd *cobra.Command, args []string) error {
	var agent *core.Agent

	var err error

	agent, err = resolveAgent(args[0])
	if err != nil {
		return err
	}

	err = core.AgentDelete(agent.Id)
	if err != nil {
		return err
	}

	fmt.Printf("agent %s removed\n", agent.Id)

	return nil
}
