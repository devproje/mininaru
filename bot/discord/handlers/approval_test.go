// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
)

func TestApprovalViewExplainsFileWriteImpact(t *testing.T) {
	var view approvalPresentation

	view = approvalView(modules.Def{Name: "file_write", Description: "raw tool description"},
		`{"path":"notes.md","content":"hello","append":false}`)

	if view.title != "Change a local file" || view.target != "notes.md" {
		t.Fatalf("approval view = %#v", view)
	}
	if !strings.Contains(view.impact, "file on disk") {
		t.Fatalf("approval impact = %q", view.impact)
	}
	if !strings.Contains(view.details, "\n  \"path\"") {
		t.Fatalf("approval details are not formatted JSON: %q", view.details)
	}
}

func TestApprovalViewEscapesCodeFenceAndBoundsDetails(t *testing.T) {
	var arguments string
	var view approvalPresentation

	arguments = `{"command":"` + "```" + strings.Repeat("x", approvalDetailsLimit+200) + `"}`
	view = approvalView(modules.Def{Name: "bash_exec"}, arguments)

	if strings.Contains(view.details, "```") {
		t.Fatalf("approval details contain a raw code fence: %q", view.details)
	}
	if len([]rune(view.details)) > approvalDetailsLimit+1 {
		t.Fatalf("approval details = %d runes, want at most %d", len([]rune(view.details)), approvalDetailsLimit+1)
	}
}

func TestApprovalCardNamesOneTimeDecisionAndExpiry(t *testing.T) {
	var components any
	var encoded []byte
	var text string

	var err error

	components = approvalRequestComponents("request-1", "user-1", approvalPresentation{
		title: "Run a shell command", target: "make test", impact: "This can change files.", details: `{}`,
	})
	encoded, err = json.Marshal(components)
	if err != nil {
		t.Fatal(err)
	}
	text = string(encoded)
	if !strings.Contains(text, "Approve once") || !strings.Contains(text, "expires in 5 minutes") {
		t.Fatalf("approval card does not explain scope and expiry: %s", text)
	}
	if strings.Contains(text, "reset:") {
		t.Fatalf("approval card unexpectedly contains reset controls: %s", text)
	}
}

func TestExpiredApprovalCardHasNoActiveControls(t *testing.T) {
	var encoded []byte
	var text string

	var err error

	encoded, err = json.Marshal(approvalClosedComponents("expired"))
	if err != nil {
		t.Fatal(err)
	}
	text = string(encoded)
	if !strings.Contains(text, "Approval expired") || strings.Contains(text, "custom_id") {
		t.Fatalf("expired approval card = %s", text)
	}
}
