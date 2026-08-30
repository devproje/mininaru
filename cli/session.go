// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"

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

func remoteSessions(cmd *cobra.Command, agentRef string) ([]*core.Session, error) {
	var id string
	var list []*core.Session
	var part []*core.Session
	var agents []*core.Agent
	var one *core.Agent

	var err error

	if agentRef != "" {
		id, _, err = remoteResolveId(cmd, "/agents", agentRef)
		if err != nil {
			return nil, err
		}

		_, err = remoteGet(cmd, "/sessions?agent_id="+url.QueryEscape(id), &list)

		return list, err
	}

	_, err = remoteGet(cmd, "/agents", &agents)
	if err != nil {
		return nil, err
	}

	for _, one = range agents {
		part = nil

		_, err = remoteGet(cmd, "/sessions?agent_id="+url.QueryEscape(one.Id), &part)
		if err != nil {
			return nil, err
		}

		list = append(list, part...)
	}

	return list, nil
}

func sessionListExecute(cmd *cobra.Command, args []string) error {
	var remote bool
	var list []*core.Session
	var agent *core.Agent
	var session *core.Session

	var err error

	remote, err = remoteEnabled(cmd)
	if err != nil {
		return err
	}

	if remote {
		list, err = remoteSessions(cmd, sessionListAgentRef)
	} else if sessionListAgentRef != "" {
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
	var remote bool
	var list []*core.Session
	var item *core.Session
	var session *core.Session

	var err error

	remote, err = remoteEnabled(cmd)
	if err != nil {
		return err
	}

	if !remote {
		session, err = resolveSession(args[0])
		if err != nil {
			return err
		}

		printSession(session)

		return nil
	}

	list, err = remoteSessions(cmd, "")
	if err != nil {
		return err
	}

	for _, item = range list {
		if item.Id == args[0] || item.Name == args[0] {
			printSession(item)

			return nil
		}
	}

	return fmt.Errorf("session %q not found", args[0])
}

func sessionRemoveExecute(cmd *cobra.Command, args []string) error {
	var remote bool

	var err error

	remote, err = remoteDo(cmd, http.MethodDelete, "/sessions/"+args[0])
	if err != nil {
		return err
	}

	if remote {
		fmt.Printf("session %s removed\n", args[0])

		return nil
	}

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
