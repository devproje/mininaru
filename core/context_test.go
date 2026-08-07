package core

import "testing"

func TestTrimHistoryKeepsNewestCompleteTurns(t *testing.T) {
	var history []*Message
	var got []*Message

	history = []*Message{
		{Role: "user", Content: "1111"},
		{Role: "assistant", Content: "2222"},
		{Role: "user", Content: "3333"},
		{Role: "assistant", Content: "4444"},
	}

	got = trimHistory(history, nil, 12, 4)
	if len(got) != 2 || got[0].Content != "3333" || got[1].Content != "4444" {
		t.Fatalf("trimmed history = %#v, want only the newest complete turn", got)
	}
}

func TestTrimHistoryAllowsEmptyHistoryWhenPromptUsesBudget(t *testing.T) {
	var history []*Message
	var got []*Message

	history = []*Message{{Role: "user", Content: "old"}, {Role: "assistant", Content: "reply"}}
	got = trimHistory(history, nil, 8, 8)
	if len(got) != 0 {
		t.Fatalf("trimmed history has %d messages, want none", len(got))
	}
}

func TestTrimHistoryChargesToolResultsToTheBudget(t *testing.T) {
	var history []*Message
	var calls map[string][]*ToolCall
	var got []*Message

	history = []*Message{
		{Id: "u1", Role: "user", Content: "1111"},
		{Role: "assistant", Content: "2222"},
		{Id: "u2", Role: "user", Content: "3333"},
		{Role: "assistant", Content: "4444"},
	}
	calls = map[string][]*ToolCall{
		"u2": {{CallId: "c1", Status: MessageCompleted, Name: "ab", Arguments: "cd", Result: "efgh"}},
	}

	got = trimHistory(history, calls, 12, 4)
	if len(got) != 0 {
		t.Fatalf("trimmed history = %#v, want none once the tool result is charged", got)
	}

	got = trimHistory(history, nil, 12, 4)
	if len(got) != 2 {
		t.Fatalf("without tool costs the newest turn should still fit, got %d messages", len(got))
	}
}
