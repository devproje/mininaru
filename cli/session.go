// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/devproje/mininaru/core"
	"github.com/spf13/cobra"
)

var sessionCmd *cobra.Command = &cobra.Command{
	Use:   "session",
	Short: "inspect and remove sessions",
}

var sessionListCmd *cobra.Command = &cobra.Command{
	Use:   "list",
	Short: "list sessions",
	RunE:  sessionListExecute,
}

var sessionShowCmd *cobra.Command = &cobra.Command{
	Use:   "show <id>",
	Short: "show a session",

	Args: cobra.ExactArgs(1),
	RunE: sessionShowExecute,
}

var sessionRemoveCmd *cobra.Command = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm", "delete"},
	Short:   "remove a session",

	Args: cobra.ExactArgs(1),
	RunE: sessionRemoveExecute,
}

var sessionListAgentRef string

func init() {
	sessionListCmd.Flags().StringVar(&sessionListAgentRef, "agent", "", "only list sessions belonging to this agent (id or name)")

	sessionCmd.AddCommand(sessionListCmd, sessionShowCmd, sessionRemoveCmd)
}

func resolveSession(id string) (*core.Session, error) {
	var session *core.Session

	var err error

	session, err = core.SessionRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session %q not found", id)
		}

		return nil, err
	}

	return session, nil
}

func printSession(session *core.Session) {
	var name string

	name = session.Name
	if name == "" {
		name = "(unnamed)"
	}

	fmt.Printf("%s  %s\n", session.Id, name)
	fmt.Printf("  agent_id    %s\n", session.AgentId)
	fmt.Printf("  created_at  %s\n", session.CreatedAt)
}

func sessionListExecute(cmd *cobra.Command, args []string) error {
	var agent *core.Agent
	var list []*core.Session
	var session *core.Session

	var err error

	if sessionListAgentRef != "" {
		agent, err = resolveAgent(sessionListAgentRef)
		if err != nil {
			return err
		}

		list, err = core.SessionList(agent.Id)
	} else {
		list, err = core.SessionListAll()
	}
	if err != nil {
		return err
	}

	if len(list) == 0 {
		fmt.Println("no sessions found")
		return nil
	}

	for _, session = range list {
		printSession(session)
	}

	return nil
}

func sessionShowExecute(cmd *cobra.Command, args []string) error {
	var session *core.Session

	var err error

	session, err = resolveSession(args[0])
	if err != nil {
		return err
	}

	printSession(session)

	return nil
}

func sessionRemoveExecute(cmd *cobra.Command, args []string) error {
	var err error

	_, err = resolveSession(args[0])
	if err != nil {
		return err
	}

	err = core.SessionDelete(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("session %s removed\n", args[0])

	return nil
}
