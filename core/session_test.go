// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"
)

func TestSessionCreateRequiresFields(t *testing.T) {
	var err error

	setupTestDB(t)

	err = SessionCreate(&Session{Id: "s1"})
	if err == nil {
		t.Fatal("expected an error for a missing agent_id")
	}
}

func TestSessionCreateRejectsUnknownAgent(t *testing.T) {
	var err error

	setupTestDB(t)

	err = SessionCreate(&Session{Id: "s1", AgentId: "missing"})
	if err == nil {
		t.Fatal("expected a foreign key violation for an unknown agent")
	}
}

func TestSessionCRUD(t *testing.T) {
	var got *Session

	var err error

	setupTestDB(t)

	err = AgentCreate(&Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	err = SessionCreate(&Session{Id: "s1", AgentId: "a1", Name: "first chat"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err = SessionRead("s1")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.AgentId != "a1" || got.Name != "first chat" {
		t.Fatalf("read = %+v, unexpected values", got)
	}

	err = SessionUpdate("s1", &Session{Name: "renamed"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err = SessionRead("s1")
	if err != nil {
		t.Fatalf("read after update failed: %v", err)
	}
	if got.Name != "renamed" {
		t.Fatalf("name = %q after update, want %q", got.Name, "renamed")
	}

	err = SessionDelete("s1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = SessionRead("s1")
	if err == nil {
		t.Fatal("expected an error reading a deleted session")
	}
}

func TestSessionListFiltersByAgent(t *testing.T) {
	var list []*Session

	var err error

	setupTestDB(t)

	err = AgentCreate(&Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}
	err = AgentCreate(&Agent{Id: "a2", Name: "coder", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	err = SessionCreate(&Session{Id: "s1", AgentId: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	err = SessionCreate(&Session{Id: "s2", AgentId: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	err = SessionCreate(&Session{Id: "s3", AgentId: "a2"})
	if err != nil {
		t.Fatal(err)
	}

	list, err = SessionList("a1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d sessions, want 2", len(list))
	}
}
