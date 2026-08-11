// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"

	"github.com/devproje/mininaru/modules"
)

func TestToolLabelNamesTheLoadedSkill(t *testing.T) {
	var label string

	label = ToolLabel(modules.SkillToolName, `{"name":"pr-review"}`)
	if label != "skill - pr-review" {
		t.Fatalf("unexpected label: %q", label)
	}

	label = ToolLabel(modules.SkillToolName, `{"name":"pr-review","path":"checklist.md"}`)
	if label != "skill - pr-review/checklist.md" {
		t.Fatalf("unexpected companion label: %q", label)
	}
}

func TestToolLabelFallsBackToTheToolName(t *testing.T) {
	var arguments string
	var label string

	for _, arguments = range []string{"", "not json", "{}", `{"name":"   "}`, `{"name":123}`} {
		label = ToolLabel(modules.SkillToolName, arguments)
		if label != modules.SkillToolName {
			t.Fatalf("arguments %q produced %q instead of a fallback", arguments, label)
		}
	}
}

func TestToolLabelLeavesOtherToolsAlone(t *testing.T) {
	var label string

	label = ToolLabel("file_read", `{"name":"pr-review","path":"x"}`)
	if label != "file_read" {
		t.Fatalf("a non skill tool was relabelled: %q", label)
	}
}
