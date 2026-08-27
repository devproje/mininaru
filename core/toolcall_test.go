// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"
)

func setupTestMessage(t *testing.T) {
	var err error

	t.Helper()

	setupTestSession(t)

	err = MessageCreate(&Message{Id: "m1", SessionId: "s1", Role: "user", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestToolCallCreateRequiresFields(t *testing.T) {
	var err error

	setupTestDB(t)
	setupTestMessage(t)

	err = ToolCallCreate(&ToolCall{Id: "t1", MessageId: "m1", CallId: "call_1"})
	if err == nil {
		t.Fatal("expected an error for a missing name")
	}
}

func TestToolCallCRUD(t *testing.T) {
	var got *ToolCall

	var err error

	setupTestDB(t)
	setupTestMessage(t)

	err = ToolCallCreate(&ToolCall{Id: "t1", MessageId: "m1", CallId: "call_1", Name: "bash_exec", Arguments: `{"command":"echo hi"}`})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err = ToolCallRead("t1")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.Status != "pending" {
		t.Fatalf("status = %q, want the schema default %q", got.Status, "pending")
	}

	err = ToolCallUpdate("t1", &ToolCall{Result: "hi", Status: "completed"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err = ToolCallRead("t1")
	if err != nil {
		t.Fatalf("read after update failed: %v", err)
	}
	if got.Result != "hi" || got.Status != "completed" {
		t.Fatalf("read after update = %+v, unexpected values", got)
	}
}

func TestToolCallListOrdersByCreation(t *testing.T) {
	var list []*ToolCall

	var err error

	setupTestDB(t)
	setupTestMessage(t)

	err = ToolCallCreate(&ToolCall{Id: "t1", MessageId: "m1", CallId: "call_1", Name: "bash_exec", Arguments: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	err = ToolCallCreate(&ToolCall{Id: "t2", MessageId: "m1", CallId: "call_2", Name: "file_read", Arguments: "{}"})
	if err != nil {
		t.Fatal(err)
	}

	list, err = ToolCallList("m1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d tool calls, want 2", len(list))
	}
	if list[0].Id != "t1" || list[1].Id != "t2" {
		t.Fatalf("list order = [%s, %s], want [t1, t2]", list[0].Id, list[1].Id)
	}
}

func TestToolCallCascadeDeleteWithMessage(t *testing.T) {
	var err error

	setupTestDB(t)
	setupTestMessage(t)

	err = ToolCallCreate(&ToolCall{Id: "t1", MessageId: "m1", CallId: "call_1", Name: "bash_exec", Arguments: "{}"})
	if err != nil {
		t.Fatal(err)
	}

	err = MessageDelete("m1")
	if err != nil {
		t.Fatalf("message delete failed: %v", err)
	}

	_, err = ToolCallRead("t1")
	if err == nil {
		t.Fatal("expected the tool call to be gone after its message was deleted")
	}
}
