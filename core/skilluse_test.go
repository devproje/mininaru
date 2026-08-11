// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/devproje/mininaru/config"
	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
)

func skillUseSetup(t *testing.T) []modules.Def {
	var root string
	var bundle string

	var err error

	t.Helper()

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}

	root = t.TempDir()
	bundle = filepath.Join(root, "pr-review")

	err = os.MkdirAll(bundle, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(bundle, "SKILL.md"),
		[]byte("---\nname: pr-review\ndescription: Review a pull request.\n---\n\nread the tests first\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = modules.SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { modules.SkillInitAt(t.TempDir(), "") })

	return []modules.Def{modules.SkillLoad()}
}

type skillUseRow struct {
	Skill   string
	Scope   string
	Path    string
	Rel     string
	Session string
}

func skillUseRows(t *testing.T) []skillUseRow {
	var rows *sql.Rows
	var stored []skillUseRow
	var row skillUseRow

	var err error

	t.Helper()

	rows, err = util.DB.Query("SELECT skill, scope, path, rel, session_id FROM skill_uses ORDER BY rowid ASC;")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&row.Skill, &row.Scope, &row.Path, &row.Rel, &row.Session)
		if err != nil {
			t.Fatal(err)
		}

		stored = append(stored, row)
	}

	return stored
}

func TestSkillUseRecordsScopeAndPath(t *testing.T) {
	var defs []modules.Def
	var record *ToolCall
	var uses []skillUseRow

	var err error

	defs = skillUseSetup(t)

	record = &ToolCall{CallId: "call-1", Name: modules.SkillToolName, Arguments: `{"name":"pr-review"}`}

	record, err = executeTool(context.Background(), "session-1", record, defs, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != MessageCompleted {
		t.Fatalf("the skill tool failed: %s", record.Error)
	}

	uses = skillUseRows(t)
	if len(uses) != 1 {
		t.Fatalf("expected one recorded use, got %d", len(uses))
	}
	if uses[0].Skill != "pr-review" {
		t.Fatalf("wrong skill recorded: %q", uses[0].Skill)
	}
	if uses[0].Scope != modules.ScopeProject {
		t.Fatalf("scope was not resolved: %q", uses[0].Scope)
	}
	if uses[0].Path == "" {
		t.Fatal("path was not resolved")
	}
}

func TestSkillUseRecordsStatelessCalls(t *testing.T) {
	var defs []modules.Def
	var record *ToolCall
	var uses []skillUseRow

	var err error

	defs = skillUseSetup(t)

	record = &ToolCall{CallId: "call-1", Name: modules.SkillToolName, Arguments: `{"name":"pr-review"}`}

	_, err = executeTool(context.Background(), "", record, defs, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	uses = skillUseRows(t)
	if len(uses) != 1 {
		t.Fatalf("a session-less call was not recorded, got %d rows", len(uses))
	}
	if uses[0].Session != "" {
		t.Fatalf("session id should be empty, got %q", uses[0].Session)
	}
}

func TestSkillUseSkipsFailedLoads(t *testing.T) {
	var defs []modules.Def
	var record *ToolCall

	var err error

	defs = skillUseSetup(t)

	record = &ToolCall{CallId: "call-1", Name: modules.SkillToolName, Arguments: `{"name":"does-not-exist"}`}

	record, err = executeTool(context.Background(), "session-1", record, defs, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != MessageFailed {
		t.Fatal("loading an unknown skill should fail")
	}

	if len(skillUseRows(t)) != 0 {
		t.Fatal("a failed load was recorded")
	}
}

func TestSkillUseRecordsCompanionReads(t *testing.T) {
	var defs []modules.Def
	var record *ToolCall
	var uses []skillUseRow

	var err error

	defs = skillUseSetup(t)

	err = os.WriteFile(filepath.Join(modules.SkillFind("pr-review").Path, "checklist.md"), []byte("check"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	record = &ToolCall{CallId: "call-1", Name: modules.SkillToolName, Arguments: `{"name":"pr-review","path":"checklist.md"}`}

	_, err = executeTool(context.Background(), "session-1", record, defs, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	uses = skillUseRows(t)
	if len(uses) != 1 {
		t.Fatalf("expected one recorded use, got %d", len(uses))
	}
	if uses[0].Rel != "checklist.md" {
		t.Fatalf("the companion path was not recorded: %q", uses[0].Rel)
	}
}

func TestSkillUseSurvivesAMissingDatabase(t *testing.T) {
	var defs []modules.Def
	var record *ToolCall
	var restore *sql.DB

	var err error

	defs = skillUseSetup(t)

	restore = util.DB
	util.DB = nil
	t.Cleanup(func() { util.DB = restore })

	record = &ToolCall{CallId: "call-1", Name: modules.SkillToolName, Arguments: `{"name":"pr-review"}`}

	record, err = executeTool(context.Background(), "session-1", record, defs, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != MessageCompleted {
		t.Fatalf("the turn failed without a database: %s", record.Error)
	}
}

func TestSkillUseRecordsThroughTheChatPath(t *testing.T) {
	var srv *httptest.Server
	var requests int
	var session *Session
	var agent *NaruAgent
	var root string
	var bundle string
	var labels []string
	var event ToolEvent
	var events []ToolEvent
	var rows []skillUseRow

	var err error

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")

		if requests == 1 {
			io.WriteString(w, toolChunk("r1", `{"role":"assistant","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"skill","arguments":"{\"name\":\"pr-review\"}"}}]}`, `"tool_calls"`))
		} else {
			io.WriteString(w, toolChunk("r2", `{"role":"assistant","content":"done"}`, `"stop"`))
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	session, agent = thinkingSetup(t, srv.URL)

	config.Client.Tools.Enabled = true
	t.Cleanup(func() { config.Client.Tools.Enabled = false })

	root = t.TempDir()
	bundle = filepath.Join(root, "pr-review")

	err = os.MkdirAll(bundle, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(bundle, "SKILL.md"),
		[]byte("---\nname: pr-review\ndescription: Review a pull request.\n---\n\nread the tests first\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = modules.SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { modules.SkillInitAt(t.TempDir(), "") })

	_, err = ChatWithApproval(context.Background(), session, agent, "review it",
		func(string) {}, func(string) {}, func(e ToolEvent) { events = append(events, e) }, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, event = range events {
		labels = append(labels, ToolLabel(event.Name, event.Arguments))
	}

	if len(labels) != 2 || labels[0] != "skill - pr-review" || labels[1] != "skill - pr-review" {
		t.Fatalf("the front end would not name the skill: %v", labels)
	}

	rows = skillUseRows(t)
	if len(rows) != 1 {
		t.Fatalf("expected one recorded use, got %d", len(rows))
	}
	if rows[0].Session != session.Id {
		t.Fatalf("the chat path did not record its session: got %q want %q", rows[0].Session, session.Id)
	}
	if rows[0].Scope != modules.ScopeProject {
		t.Fatalf("scope was not resolved: %q", rows[0].Scope)
	}
}

func TestSkillUseStatsAggregatesByCount(t *testing.T) {
	var defs []modules.Def
	var record *ToolCall
	var index int
	var uses []*SkillUse

	var err error

	defs = skillUseSetup(t)

	for index = 0; index < 3; index++ {
		record = &ToolCall{CallId: "call-1", Name: modules.SkillToolName, Arguments: `{"name":"pr-review"}`}

		_, err = executeTool(context.Background(), "session-1", record, defs, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	uses, err = SkillUseStats("")
	if err != nil {
		t.Fatal(err)
	}
	if len(uses) != 1 || uses[0].Count != 3 {
		t.Fatalf("unexpected aggregate: %+v", uses)
	}

	uses, err = SkillUseStats("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(uses) != 1 {
		t.Fatalf("the session filter dropped matching rows: %+v", uses)
	}

	uses, err = SkillUseStats("session-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(uses) != 0 {
		t.Fatalf("the session filter matched a different session: %+v", uses)
	}
}
