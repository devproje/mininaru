// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/devproje/mininaru/core"
)

func gatewaySessions(base string, apiKey string) ([]*core.Session, []*core.Agent, error) {
	var agents []*core.Agent
	var one *core.Agent
	var part []*core.Session
	var sessions []*core.Session

	var err error

	err = Api(http.MethodGet, base+"/agents", apiKey, nil, &agents)
	if err != nil {
		return nil, nil, err
	}

	if len(agents) == 0 {
		return nil, nil, fmt.Errorf("this gateway has no agents")
	}

	for _, one = range agents {
		part = nil

		err = Api(http.MethodGet, base+"/sessions?agent_id="+url.QueryEscape(one.Id), apiKey, nil, &part)
		if err != nil {
			return nil, nil, err
		}

		sessions = append(sessions, part...)
	}

	return sessions, agents, nil
}

func switchGateway(sh *Shell, target Gateway, session *core.Session) error {
	var base string
	var agent *core.Agent

	var err error

	base, err = ApiBase(target.Url)
	if err != nil {
		return err
	}

	agent, err = Agent(base, target.ApiKey, session.AgentId)
	if err != nil {
		return err
	}

	sh.url = target.Url
	sh.apiKey = target.ApiKey
	sh.base = base
	sh.session = session
	sh.agent = agent

	err = sh.reconnect()
	if err != nil {
		return err
	}

	write("  %s⇄%s %s %s(%s @ %s)%s\n", BLUE, RESET, sh.agent.Name, DIM, sh.session.Name, target.Name, RESET)

	return nil
}

func cmdGateway(sh *Shell, args string) error {
	var one Gateway
	var names []string
	var pick int
	var target Gateway
	var base string
	var sessions []*core.Session
	var agents []*core.Agent
	var labels []string
	var session *core.Session
	var created core.Session

	var err error

	if len(sh.gateways) == 0 {
		return fmt.Errorf("no gateways — add one with 'mininaru gateway add'")
	}

	for _, one = range sh.gateways {
		names = append(names, fmt.Sprintf("%s  %s%s%s", one.Name, DIM, one.Url, RESET))
	}

	pick, err = selectFrom("gateway", names, sh.keys)
	if errors.Is(err, errInterrupted) {
		return nil
	}
	if err != nil {
		return err
	}

	target = sh.gateways[pick]

	base, err = ApiBase(target.Url)
	if err != nil {
		return err
	}

	sessions, agents, err = gatewaySessions(base, target.ApiKey)
	if err != nil {
		return err
	}

	labels = []string{"＋ new session"}
	for _, session = range sessions {
		labels = append(labels, session.Name)
	}

	pick, err = selectFrom("session @ "+target.Name, labels, sh.keys)
	if errors.Is(err, errInterrupted) {
		return nil
	}
	if err != nil {
		return err
	}

	if pick == 0 {
		err = Api(http.MethodPost, base+"/sessions", target.ApiKey, map[string]string{"agent_id": agents[0].Id}, &created)
		if err != nil {
			return err
		}

		return switchGateway(sh, target, &created)
	}

	return switchGateway(sh, target, sessions[pick-1])
}
