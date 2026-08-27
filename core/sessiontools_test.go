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

func TestSessionListReturnsEveryLiveSessionAndMarksTheCurrentOne(t *testing.T) {
	var tool modules.Tool
	var result string
	var summaries []sessionSummary
	var caller *Agent
	var byId map[string]sessionSummary
	var one sessionSummary

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

	tool = sessionListTool(caller, "s1")

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

	if len(summaries) != 2 {
		t.Fatalf("session_list = %+v, want both live sessions across both agents", summaries)
	}

	byId = make(map[string]sessionSummary, len(summaries))
	for _, one = range summaries {
		byId[one.Id] = one
	}

	if !byId["s1"].Current || byId["s1"].Agent != "naru" {
		t.Fatalf("s1 = %+v, want current=true and agent=naru", byId["s1"])
	}
	if byId["s3"].Current || byId["s3"].Agent != "other" {
		t.Fatalf("s3 = %+v, want current=false and agent=other", byId["s3"])
	}
}
