// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
)

func skillNoteEnv(t *testing.T) {
	var err error

	t.Helper()

	skillCreateEnv(t)

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		util.DB.Close()
		util.DB = nil
	})

	_, err = SkillCreateResult("deploy", "Deploy the service.", "# Steps\n\n1. Push the tag.", "", false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSkillNoteSurfacesOnTheNextLoad(t *testing.T) {
	var def Def
	var loaded string

	var err error

	skillNoteEnv(t)

	loaded, err = SkillResult("deploy", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(loaded, skillNoteOpenTag) {
		t.Fatalf("a skill with no notes carried a note block: %q", loaded)
	}

	def = SkillNoteTool()

	_, err = def.Execute(context.Background(), `{"skill":"deploy","note":"The tag must be signed or the release job rejects it."}`)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err = SkillResult("deploy", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loaded, "The tag must be signed") {
		t.Fatalf("the note did not reach the loaded skill: %q", loaded)
	}
	if !strings.Contains(loaded, "1. Push the tag.") {
		t.Fatalf("the note block displaced the skill body: %q", loaded)
	}
}

func TestSkillNoteRejectsAnUnknownSkill(t *testing.T) {
	var def Def

	var err error

	skillNoteEnv(t)

	def = SkillNoteTool()

	_, err = def.Execute(context.Background(), `{"skill":"nope","note":"anything"}`)
	if err == nil {
		t.Fatal("a note was accepted against a skill that does not exist")
	}
}

func TestSkillNoteRecordsTheSession(t *testing.T) {
	var def Def
	var ctx context.Context
	var notes []SkillNote

	var err error

	skillNoteEnv(t)

	def = SkillNoteTool()
	ctx = SessionContext(context.Background(), "session-42")

	_, err = def.Execute(ctx, `{"skill":"deploy","note":"Run the smoke test before announcing."}`)
	if err != nil {
		t.Fatal(err)
	}

	notes, err = SkillNotesFor("deploy", true)
	if err != nil || len(notes) != 1 {
		t.Fatalf("notes=%#v err=%v", notes, err)
	}
	if notes[0].SessionId != "session-42" {
		t.Fatalf("session id was not recorded: %q", notes[0].SessionId)
	}
}

func TestSkillNoteBlockKeepsTheNewestWithinTheCap(t *testing.T) {
	var index int
	var loaded string

	var err error

	skillNoteEnv(t)

	for index = 0; index < maxPendingNotes+3; index++ {
		_, err = SkillNoteAdd("deploy", "lesson number "+strconv.Itoa(index), "")
		if err != nil {
			t.Fatal(err)
		}
	}

	loaded, err = SkillResult("deploy", "")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(loaded, "lesson number 0 ") || strings.Contains(loaded, "lesson number 0\n") {
		t.Fatalf("the oldest note past the cap was still rendered: %q", loaded)
	}
	if !strings.Contains(loaded, "lesson number "+strconv.Itoa(maxPendingNotes+2)) {
		t.Fatalf("the newest note was dropped: %q", loaded)
	}
	if !strings.Contains(loaded, "3 older observations are not shown") {
		t.Fatalf("the elision was not reported: %q", loaded)
	}
}

func TestSkillReviseFoldsNotesAndKeepsHistory(t *testing.T) {
	var def Def
	var result string
	var pending []SkillNote
	var all []SkillNote
	var revisions []SkillRevision
	var loaded string

	var err error

	skillNoteEnv(t)

	_, err = SkillNoteAdd("deploy", "The tag must be signed.", "")
	if err != nil {
		t.Fatal(err)
	}

	def = SkillRevise()

	result, err = def.Execute(context.Background(),
		`{"name":"deploy","body":"# Steps\n\n1. Sign the tag.\n2. Push the tag.","reason":"fold in the signing rule"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "notes folded in: 1") {
		t.Fatalf("the revision did not consume the pending note: %q", result)
	}

	pending, err = SkillNotesFor("deploy", true)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending=%#v err=%v", pending, err)
	}

	all, err = SkillNotesFor("deploy", false)
	if err != nil || len(all) != 1 || !all[0].Applied {
		t.Fatalf("the note was not kept as applied: %#v err=%v", all, err)
	}

	loaded, err = SkillResult("deploy", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loaded, "1. Sign the tag.") {
		t.Fatalf("the revised body did not load back: %q", loaded)
	}
	if strings.Contains(loaded, skillNoteOpenTag) {
		t.Fatalf("an applied note was still shown as pending: %q", loaded)
	}

	revisions, err = SkillRevisions("deploy")
	if err != nil || len(revisions) != 1 {
		t.Fatalf("revisions=%#v err=%v", revisions, err)
	}
	if !strings.Contains(revisions[0].Body, "1. Push the tag.") {
		t.Fatalf("the snapshot did not keep the previous body: %q", revisions[0].Body)
	}
	if revisions[0].Reason != "fold in the signing rule" {
		t.Fatalf("the reason was not stored: %q", revisions[0].Reason)
	}
}

func TestSkillReviseKeepsTheDescriptionWhenOmitted(t *testing.T) {
	var entry *Skill

	var err error

	skillNoteEnv(t)

	_, err = SkillReviseResult("deploy", "", "# Steps\n\n1. Sign the tag.", "")
	if err != nil {
		t.Fatal(err)
	}

	entry = SkillFind("deploy")
	if entry == nil || entry.Description != "Deploy the service." {
		t.Fatalf("the description did not survive the revision: %#v", entry)
	}
}

func TestSkillReviseRejectsAnUnknownSkill(t *testing.T) {
	var err error

	skillNoteEnv(t)

	_, err = SkillReviseResult("nope", "", "# Steps", "")
	if err == nil {
		t.Fatal("a revision was accepted for a skill that does not exist")
	}
}

func TestSkillRestoreUndoesARevision(t *testing.T) {
	var revisions []SkillRevision
	var entry *Skill

	var err error

	skillNoteEnv(t)

	_, err = SkillReviseResult("deploy", "", "# Steps\n\n1. Something worse.", "a bad idea")
	if err != nil {
		t.Fatal(err)
	}

	revisions, err = SkillRevisions("deploy")
	if err != nil || len(revisions) != 1 {
		t.Fatalf("revisions=%#v err=%v", revisions, err)
	}

	_, err = SkillRestore("deploy", "")
	if err != nil {
		t.Fatal(err)
	}

	entry = SkillFind("deploy")
	if entry == nil || !strings.Contains(entry.Body, "1. Push the tag.") {
		t.Fatalf("the rollback did not restore the earlier body: %#v", entry)
	}

	revisions, err = SkillRevisions("deploy")
	if err != nil || len(revisions) != 2 {
		t.Fatalf("the rolled-back version was not itself snapshotted: %#v err=%v", revisions, err)
	}
}

func TestSkillRestoreReportsAnEmptyHistory(t *testing.T) {
	var err error

	skillNoteEnv(t)

	_, err = SkillRestore("deploy", "")
	if err == nil {
		t.Fatal("a rollback succeeded against a skill with no history")
	}
}

func TestSafeToolsIncludeSkillNoteButNotRevise(t *testing.T) {
	var def Def
	var note bool

	for _, def = range SafeTools() {
		if def.Name == SkillReviseToolName {
			t.Fatal("safe daemon tools exposed privileged skill_revise")
		}
		if def.Name == SkillNoteToolName {
			note = true
		}
	}

	if !note {
		t.Fatal("skill_note is not available to safe daemon front ends")
	}
}
