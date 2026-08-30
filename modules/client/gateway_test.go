// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devproje/mininaru/core"
)

func TestGatewaySessionsSweepsEveryAgent(t *testing.T) {
	var srv *httptest.Server
	var sessions []*core.Session
	var agents []*core.Agent
	var seen []string

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path+"?"+r.URL.RawQuery)

		if r.URL.Path == "/api/agents" {
			json.NewEncoder(w).Encode([]*core.Agent{{Id: "a1", Name: "one"}, {Id: "a2", Name: "two"}})
			return
		}

		switch r.URL.Query().Get("agent_id") {
		case "a1":
			json.NewEncoder(w).Encode([]*core.Session{{Id: "s1", Name: "alpha", AgentId: "a1"}})
		case "a2":
			json.NewEncoder(w).Encode([]*core.Session{{Id: "s2", Name: "beta", AgentId: "a2"}, {Id: "s3", Name: "gamma", AgentId: "a2"}})
		}
	}))
	defer srv.Close()

	sessions, agents, err = gatewaySessions(srv.URL+"/api", "sk-x")
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 2 {
		t.Fatalf("agents = %d", len(agents))
	}

	if len(sessions) != 3 || sessions[0].Name != "alpha" || sessions[2].Name != "gamma" {
		t.Fatalf("sessions = %+v", sessions)
	}

	if !strings.Contains(strings.Join(seen, " "), "agent_id=a1") || !strings.Contains(strings.Join(seen, " "), "agent_id=a2") {
		t.Fatalf("did not sweep both agents: %v", seen)
	}
}

func TestGatewaySessionsErrorsWithNoAgents(t *testing.T) {
	var srv *httptest.Server

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*core.Agent{})
	}))
	defer srv.Close()

	_, _, err = gatewaySessions(srv.URL+"/api", "")
	if err == nil {
		t.Fatal("want error for a gateway with no agents")
	}
}
