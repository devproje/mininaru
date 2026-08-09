// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
	"github.com/openai/openai-go"
)

func privilegedDef(executions *int) []modules.Def {
	return []modules.Def{{
		Name: "memory", Permission: modules.PermissionPrivileged,
		Execute: func(context.Context, string) (string, error) {
			*executions++
			return "executed", nil
		},
	}}
}

func TestPrivilegedToolRefusedWithoutInteractiveFrontEnd(t *testing.T) {
	var defs []modules.Def
	var executions int
	var call openai.ChatCompletionMessageToolCall
	var record *ToolCall
	var approvals int

	var err error

	defs = privilegedDef(&executions)

	call = openai.ChatCompletionMessageToolCall{ID: "call-privileged"}
	call.Function.Name = "memory"
	call.Function.Arguments = `{"action":"list"}`

	record, err = toolCallStart("", call)
	if err != nil {
		t.Fatal(err)
	}

	record, err = executeTool(context.Background(), record, defs, true, false,
		func(context.Context, modules.Def, string) (bool, error) {
			approvals++
			return true, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	if executions != 0 {
		t.Fatalf("privileged tool ran %d times over an untrusted front end", executions)
	}
	if approvals != 0 {
		t.Fatalf("privileged tool asked for approval %d times, it should be refused outright", approvals)
	}
	if record.Status != MessageFailed {
		t.Fatalf("record status = %q, want %q", record.Status, MessageFailed)
	}
	if !strings.Contains(record.Result, "interactive front end") {
		t.Fatalf("result = %q, want the privileged refusal", record.Result)
	}
}

func TestPrivilegedToolRunsForInteractiveFrontEnd(t *testing.T) {
	var defs []modules.Def
	var executions int
	var call openai.ChatCompletionMessageToolCall
	var record *ToolCall
	var approvals int

	var err error

	defs = privilegedDef(&executions)

	call = openai.ChatCompletionMessageToolCall{ID: "call-privileged"}
	call.Function.Name = "memory"
	call.Function.Arguments = `{"action":"list"}`

	record, err = toolCallStart("", call)
	if err != nil {
		t.Fatal(err)
	}

	record, err = executeTool(context.Background(), record, defs, false, true,
		func(context.Context, modules.Def, string) (bool, error) {
			approvals++
			return true, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	if executions != 1 {
		t.Fatalf("privileged tool ran %d times, want 1", executions)
	}
	if approvals != 0 {
		t.Fatalf("privileged tool asked for approval %d times, want 0", approvals)
	}
	if record.Result != "executed" {
		t.Fatalf("result = %q, want executed", record.Result)
	}
}

func TestSafeToolsCarryNoPrivilegedTool(t *testing.T) {
	var def modules.Def

	for _, def = range modules.SafeTools() {
		if def.Permission == modules.PermissionSafe {
			continue
		}

		t.Fatalf("SafeTools exposed %q with permission %s", def.Name, def.Permission)
	}
}
