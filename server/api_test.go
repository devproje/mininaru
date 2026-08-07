package server

import (
	"encoding/json"
	"testing"
)

func TestContentTextAcceptsStringAndParts(t *testing.T) {
	var cases []struct {
		raw  string
		want string
	}
	var cur struct {
		raw  string
		want string
	}
	var got string

	cases = []struct {
		raw  string
		want string
	}{
		{`"hello"`, "hello"},
		{`[{"type":"text","text":"hello"},{"type":"text","text":" world"}]`, "hello world"},
		{`[{"type":"text","text":"only"},{"type":"image_url","image_url":{"url":"http://x"}}]`, "only"},
		{`null`, ""},
		{`""`, ""},
		{`123`, ""},
	}

	for _, cur = range cases {
		got = contentText(json.RawMessage(cur.raw))
		if got != cur.want {
			t.Fatalf("contentText(%s) = %q, want %q", cur.raw, got, cur.want)
		}
	}
}

func TestRequestMessagesMapRoles(t *testing.T) {
	var messages []RequestMessage
	var converted []any
	var buf []byte

	var err error

	messages = []RequestMessage{
		{Role: roleSystem, Content: json.RawMessage(`"sys"`)},
		{Role: roleAssistant, Content: json.RawMessage(`"reply"`)},
		{Role: roleUser, Content: json.RawMessage(`[{"type":"text","text":"ask"}]`)},
	}

	buf, err = json.Marshal(requestMessages(messages))
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(buf, &converted)
	if err != nil {
		t.Fatal(err)
	}

	if len(converted) != 3 {
		t.Fatalf("converted %d messages, want 3", len(converted))
	}

	if !containsAll(string(buf), `"role":"system"`, `"sys"`, `"role":"assistant"`, `"reply"`, `"role":"user"`, `"ask"`) {
		t.Fatalf("converted messages = %s", buf)
	}
}
