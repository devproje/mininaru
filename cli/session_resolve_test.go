// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

func seed(t *testing.T) (*core.Session, *core.Session) {
	var older, newer *core.Session

	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	core.Global = &core.NaruAgent{Id: "agent-1", Name: "naru"}
	sessionIdRef = ""

	older, err = core.SessionCreate(core.Global, "older")
	if err != nil {
		t.Fatal(err)
	}
	_, err = core.MessageSave(older.Id, "user", "a", "")
	if err != nil {
		t.Fatal(err)
	}

	newer, err = core.SessionCreate(core.Global, "newer")
	if err != nil {
		t.Fatal(err)
	}
	_, err = core.MessageSave(newer.Id, "user", "b", "")
	if err != nil {
		t.Fatal(err)
	}

	return older, newer
}

func TestResolveSessionBareFlagPicksLatest(t *testing.T) {
	var newer, got *core.Session

	var err error

	_, newer = seed(t)
	sessionIdRef = latestSession

	got, err = resolveSession(core.Global, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Id != newer.Id {
		t.Fatalf("resolved %q, want latest %q", got.Name, newer.Name)
	}
}

func TestResolveSessionPositionalWinsOverLatest(t *testing.T) {
	var older, got *core.Session

	var err error

	older, _ = seed(t)
	sessionIdRef = latestSession

	got, err = resolveSession(core.Global, []string{older.Id})
	if err != nil {
		t.Fatal(err)
	}
	if got.Id != older.Id {
		t.Fatalf("resolved %q, want the explicitly named %q", got.Name, older.Name)
	}
}

func TestResolveSessionNoFlagCreatesNew(t *testing.T) {
	var older, newer, got *core.Session

	var err error

	older, newer = seed(t)

	got, err = resolveSession(core.Global, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Id == older.Id || got.Id == newer.Id {
		t.Fatalf("reused existing session %q, want a brand new one", got.Name)
	}
}

func TestResolveSessionRejectsDifferentAgent(t *testing.T) {
	var foreign *core.Session

	var err error

	seed(t)
	foreign, err = core.SessionCreate(&core.NaruAgent{Id: "agent-2"}, "foreign")
	if err != nil {
		t.Fatal(err)
	}
	sessionIdRef = foreign.Id

	_, err = resolveSession(core.Global, nil)
	if err == nil {
		t.Fatal("foreign-agent session unexpectedly resolved")
	}
}

func TestResolveAgentPrefersFlagOverGlobal(t *testing.T) {
	var helper, got *core.NaruAgent

	var err error

	seed(t)
	helper = &core.NaruAgent{Id: "agent-2", Name: "helper"}
	core.Agents = []*core.NaruAgent{helper}
	chatAgentRef = ""

	got, err = resolveAgent()
	if err != nil || got != core.Global {
		t.Fatalf("default agent = %#v err=%v", got, err)
	}

	chatAgentRef = "helper"

	got, err = resolveAgent()
	if err != nil || got != helper {
		t.Fatalf("named agent = %#v err=%v", got, err)
	}

	chatAgentRef = "ghost"

	_, err = resolveAgent()
	if err == nil {
		t.Fatal("unknown agent name resolved")
	}

	chatAgentRef = ""
}

func TestResolveSessionScopesToRequestedAgent(t *testing.T) {
	var globalSession *core.Session
	var helper *core.NaruAgent
	var got *core.Session

	var err error

	_, globalSession = seed(t)
	helper = &core.NaruAgent{Id: "agent-2", Name: "helper"}
	core.Agents = []*core.NaruAgent{helper}

	sessionIdRef = globalSession.Id

	_, err = resolveSession(helper, nil)
	if err == nil {
		t.Fatal("helper resolved a session owned by the global agent")
	}

	sessionIdRef = ""

	got, err = resolveSession(helper, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentId != helper.Id {
		t.Fatalf("new session belongs to %q, want %q", got.AgentId, helper.Id)
	}
}
