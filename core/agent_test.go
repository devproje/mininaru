// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"
)

func TestAgentCreateRequiresFields(t *testing.T) {
	var err error

	setupTestDB(t)

	err = ProviderCreate(&Provider{Id: "p1", Name: "one"})
	if err != nil {
		t.Fatal(err)
	}

	err = AgentCreate(&Agent{Id: "a1", Name: "naru"})
	if err == nil {
		t.Fatal("expected an error for a missing model")
	}
}

func TestAgentCRUD(t *testing.T) {
	var got *Agent

	var err error

	setupTestDB(t)

	err = AgentCreate(&Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err = AgentRead("a1")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.Name != "naru" || got.Model != "gpt-4o-mini" {
		t.Fatalf("read = %+v, unexpected values", got)
	}
	if got.ThinkingLevel != "medium" {
		t.Fatalf("thinking_level = %q, want the schema default %q", got.ThinkingLevel, "medium")
	}
	if got.MaxContext != 24000 {
		t.Fatalf("max_context = %d, want the schema default 24000", got.MaxContext)
	}

	err = AgentUpdate("a1", &Agent{Soul: "a terse assistant"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err = AgentRead("a1")
	if err != nil {
		t.Fatalf("read after update failed: %v", err)
	}
	if got.Soul != "a terse assistant" {
		t.Fatalf("soul = %q after update, want %q", got.Soul, "a terse assistant")
	}

	err = AgentDelete("a1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = AgentRead("a1")
	if err == nil {
		t.Fatal("expected an error reading a deleted agent")
	}
}
