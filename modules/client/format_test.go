// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidFormat(t *testing.T) {
	var format string

	for _, format = range []string{"", "string", "json", "xml"} {
		if !ValidFormat(format) {
			t.Fatalf("%q should be valid", format)
		}
	}

	for _, format = range []string{"yaml", "toml", "JSON", "text"} {
		if ValidFormat(format) {
			t.Fatalf("%q should be invalid", format)
		}
	}
}

func TestMarshalResultJSON(t *testing.T) {
	var want Result
	var got Result
	var out []byte

	var err error

	want = Result{
		SessionId: "quiet-otter",
		Content:   "hello",
		Tools:     []ToolResult{{Name: "bash", Status: "finished"}},
	}

	out, err = marshalResult(FormatJSON, want)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(out, &got)
	if err != nil {
		t.Fatal(err)
	}

	if got.SessionId != want.SessionId || got.Content != want.Content {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	if len(got.Tools) != 1 || got.Tools[0] != want.Tools[0] {
		t.Fatalf("tools mismatch: %+v", got.Tools)
	}
}

func TestMarshalResultXML(t *testing.T) {
	var out []byte
	var text string

	var err error

	out, err = marshalResult(FormatXML, Result{SessionId: "quiet-otter", Content: "hi", Tools: []ToolResult{{Name: "bash", Status: "failed"}}})
	if err != nil {
		t.Fatal(err)
	}

	text = string(out)

	for _, want := range []string{"<result>", "<session_id>quiet-otter</session_id>", "<tool>", "<status>failed</status>", "<name>bash</name>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("xml missing %q:\n%s", want, text)
		}
	}
}

func TestMarshalResultRejectsUnknown(t *testing.T) {
	var err error

	_, err = marshalResult("string", Result{})
	if err == nil {
		t.Fatal("want error for non-structured format")
	}
}
