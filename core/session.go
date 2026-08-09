// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type Session struct {
	Id         string `json:"id"`
	AgentId    string `json:"agent_id"`
	Name       string `json:"name"`
	Origin     string `json:"origin"`
	ExternalId string `json:"external_id"`
}

const sessionColumns = "id, agent_id, name, origin, external_id"

func NewSession(agent *NaruAgent, name string) *Session {
	var session Session

	if agent == nil {
		return nil
	}

	session = Session{
		Id:      uuid.NewString(),
		AgentId: agent.Id,
		Name:    name,
	}

	return &session
}

func SessionCreate(agent *NaruAgent, name string) (*Session, error) {
	var session *Session

	var err error

	session = NewSession(agent, name)
	if session == nil {
		return nil, fmt.Errorf("agent is required to create a session")
	}

	_, err = util.DB.Exec("INSERT INTO sessions (id, agent_id, name, origin, external_id) VALUES (?, ?, ?, ?, ?);",
		session.Id, session.AgentId, session.Name, session.Origin, session.ExternalId)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func SessionFind(id string) (*Session, error) {
	var session Session

	var err error

	err = util.DB.QueryRow("SELECT "+sessionColumns+" FROM sessions WHERE id = ?;", id).
		Scan(&session.Id, &session.AgentId, &session.Name, &session.Origin, &session.ExternalId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session id %s not found", id)
		}

		return nil, err
	}

	return &session, nil
}

func SessionLatest(agentId string) (*Session, error) {
	var query string
	var session Session

	var err error

	query = `SELECT ` + sessionColumns + ` FROM sessions
	WHERE agent_id = ? AND EXISTS (SELECT 1 FROM messages WHERE messages.session_id = sessions.id AND messages.status = 'completed')
	ORDER BY rowid DESC LIMIT 1;`

	err = util.DB.QueryRow(query, agentId).Scan(&session.Id, &session.AgentId, &session.Name, &session.Origin, &session.ExternalId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &session, nil
}

func SessionList(agentId string) ([]*Session, error) {
	var rows *sql.Rows
	var cur Session
	var sessions []*Session

	var err error

	rows, err = util.DB.Query("SELECT "+sessionColumns+" FROM sessions WHERE agent_id = ?;", agentId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&cur.Id, &cur.AgentId, &cur.Name, &cur.Origin, &cur.ExternalId)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, &Session{Id: cur.Id, AgentId: cur.AgentId, Name: cur.Name,
			Origin: cur.Origin, ExternalId: cur.ExternalId})
	}

	return sessions, nil
}

func SessionByExternal(origin, externalId string) (*Session, error) {
	var session Session

	var err error

	if origin == "" || externalId == "" {
		return nil, fmt.Errorf("origin and external id are required")
	}

	err = util.DB.QueryRow("SELECT "+sessionColumns+" FROM sessions WHERE origin = ? AND external_id = ?;", origin, externalId).
		Scan(&session.Id, &session.AgentId, &session.Name, &session.Origin, &session.ExternalId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &session, nil
}

func SessionAttach(agent *NaruAgent, origin, externalId, name string) (*Session, error) {
	var session *Session
	var tx *sql.Tx

	var err error

	if agent == nil {
		return nil, fmt.Errorf("agent is required to attach a session")
	}

	if origin == "" || externalId == "" {
		return nil, fmt.Errorf("origin and external id are required")
	}

	session = NewSession(agent, name)
	session.Origin = origin
	session.ExternalId = externalId

	tx, err = util.DB.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("UPDATE sessions SET external_id = '' WHERE origin = ? AND external_id = ?;", origin, externalId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	_, err = tx.Exec("INSERT INTO sessions (id, agent_id, name, origin, external_id) VALUES (?, ?, ?, ?, ?);",
		session.Id, session.AgentId, session.Name, session.Origin, session.ExternalId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return session, nil
}

func SessionUpdate(id, name string) error {
	var result sql.Result
	var affected int64

	var err error

	result, err = util.DB.Exec("UPDATE sessions SET name = ? WHERE id = ?;", name, id)
	if err != nil {
		return err
	}

	affected, err = result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("session id %s not found", id)
	}

	return nil
}

func SessionDeleteByAgent(agentId string) error {
	var err error

	_, err = util.DB.Exec("DELETE FROM sessions WHERE agent_id = ?;", agentId)

	return err
}

func SessionDelete(id string) error {
	var result sql.Result
	var affected int64

	var err error

	result, err = util.DB.Exec("DELETE FROM sessions WHERE id = ?;", id)
	if err != nil {
		return err
	}

	affected, err = result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("session is not found, aborted.")
	}

	return nil
}
