// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"
)

func setupTestSession(t *testing.T) {
	var err error

	t.Helper()

	err = AgentCreate(&Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	err = SessionCreate(&Session{Id: "s1", AgentId: "a1"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMessageCreateRequiresFields(t *testing.T) {
	var err error

	setupTestDB(t)
	setupTestSession(t)

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user"})
	if err == nil {
		t.Fatal("expected an error for a missing content")
	}
}

func TestMessageCRUD(t *testing.T) {
	var got *Message

	var err error

	setupTestDB(t)
	setupTestSession(t)

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "hello"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err = MessageRead("m1")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.Status != "pending" {
		t.Fatalf("status = %q, want the schema default %q", got.Status, "pending")
	}

	err = MessageUpdate("m1", &Message{Content: "hi there", Status: "completed"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err = MessageRead("m1")
	if err != nil {
		t.Fatalf("read after update failed: %v", err)
	}
	if got.Content != "hi there" || got.Status != "completed" {
		t.Fatalf("read after update = %+v, unexpected values", got)
	}

	err = MessageDelete("m1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = MessageRead("m1")
	if err == nil {
		t.Fatal("expected an error reading a deleted message")
	}
}

func TestMessageListOrdersByCreation(t *testing.T) {
	var list []*Message

	var err error

	setupTestDB(t)
	setupTestSession(t)

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "first"})
	if err != nil {
		t.Fatal(err)
	}
	err = MessageCreate(&Message{Id: "m2", SessionId: "s1", Role: "assistant", Content: "second"})
	if err != nil {
		t.Fatal(err)
	}

	list, err = MessageList("s1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d messages, want 2", len(list))
	}
	if list[0].Id != "m1" || list[1].Id != "m2" {
		t.Fatalf("list order = [%s, %s], want [m1, m2]", list[0].Id, list[1].Id)
	}
}

func TestMessageCascadeDeleteWithSession(t *testing.T) {
	var err error

	setupTestDB(t)
	setupTestSession(t)

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	err = SessionDelete("s1")
	if err != nil {
		t.Fatalf("session delete failed: %v", err)
	}

	_, err = MessageRead("m1")
	if err == nil {
		t.Fatal("expected the message to be gone after its session was deleted")
	}
}
