// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
)

func TestAgentListReturnsAllConfiguredAgents(t *testing.T) {
	var tool modules.Tool
	var result string
	var summaries []agentSummary

	var err error

	setupTestDB(t)

	err = AgentCreate(&Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}
	err = AgentCreate(&Agent{Id: "a2", Name: "worker", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	tool = agentListTool()

	result, err = tool.Execute(t.Context(), "{}")
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal([]byte(result), &summaries)
	if err != nil {
		t.Fatal(err)
	}

	if len(summaries) != 2 {
		t.Fatalf("agent_list = %+v, want 2 agents", summaries)
	}
}

func TestSessionListOnlyReturnsLiveSessionsOwnedByTheCaller(t *testing.T) {
	var tool modules.Tool
	var result string
	var summaries []sessionSummary
	var caller *Agent

	var err error

	setupTestDB(t)

	caller = &Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"}
	err = AgentCreate(caller)
	if err != nil {
		t.Fatal(err)
	}

	err = AgentCreate(&Agent{Id: "a2", Name: "other", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	err = SessionCreate(&Session{Id: "s1", AgentId: "a1", Name: "live one"})
	if err != nil {
		t.Fatal(err)
	}
	err = SessionCreate(&Session{Id: "s2", AgentId: "a1", Name: "not live"})
	if err != nil {
		t.Fatal(err)
	}
	err = SessionCreate(&Session{Id: "s3", AgentId: "a2", Name: "someone else's live session"})
	if err != nil {
		t.Fatal(err)
	}

	tool = sessionListTool(caller)

	result, err = tool.Execute(t.Context(), "{}")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != "[]" && strings.TrimSpace(result) != "null" {
		t.Fatalf("session_list with nothing live = %q, want an empty result", result)
	}

	SetLiveSessionsLister(func() []string { return []string{"s1", "s3"} })
	t.Cleanup(func() { SetLiveSessionsLister(nil) })

	result, err = tool.Execute(t.Context(), "{}")
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal([]byte(result), &summaries)
	if err != nil {
		t.Fatal(err)
	}

	if len(summaries) != 1 || summaries[0].Id != "s1" {
		t.Fatalf("session_list = %+v, want only s1 (live and owned by the caller)", summaries)
	}
}
