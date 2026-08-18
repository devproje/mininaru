// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules"
	mininaruv1 "github.com/devproje/mininaru/rpc/gen/mininaru/v1"
)

func TestRemoteToolRunsOnTheClientAfterLocalApproval(t *testing.T) {
	var executed bool
	var approved bool
	var defs []modules.Def
	var result string

	var err error

	defs = []modules.Def{{Name: "local", Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			executed = true
			return arguments, nil
		}}}
	result, err = executeLocalTool(context.Background(), &mininaruv1.ToolRequest{ToolName: "local", Arguments: "client-data"}, defs,
		func(ctx context.Context, def modules.Def, arguments string) (bool, error) {
			approved = true
			return true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !approved || !executed || result != "client-data" {
		t.Fatalf("approved=%t executed=%t result=%q", approved, executed, result)
	}
}

func TestRemoteToolRefusesDangerousWorkWithoutAnApprovalUI(t *testing.T) {
	var executed bool
	var defs []modules.Def

	var err error

	defs = []modules.Def{{Name: "local", Permission: modules.PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			executed = true
			return "", nil
		}}}
	_, err = executeLocalTool(context.Background(), &mininaruv1.ToolRequest{ToolName: "local"}, defs, core.ToolApprovalFunc(nil))
	if err == nil || executed {
		t.Fatalf("err=%v executed=%t", err, executed)
	}
}
