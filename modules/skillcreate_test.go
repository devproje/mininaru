// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/util"
)

func skillCreateEnv(t *testing.T) {
	var err error

	t.Helper()

	util.RootDir = t.TempDir()
	t.Setenv("HOME", t.TempDir())

	err = SkillInit()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { SkillInitAt(t.TempDir(), "") })
}

func TestSkillCreateRoundTripsThroughTheLoader(t *testing.T) {
	var target string
	var entry *Skill

	var err error

	skillCreateEnv(t)

	_, err = SkillCreateResult("pr-review", "Review a pull request: correctness first.", "# Steps\n\n1. Read the tests.", "", false)
	if err != nil {
		t.Fatal(err)
	}

	target = filepath.Join(util.Path(SKILL_DIR), "pr-review")

	entry, err = skillParse(target, ScopeProject)
	if err != nil {
		t.Fatalf("the written bundle does not parse back: %v", err)
	}

	if entry.Name != "pr-review" {
		t.Fatalf("name did not survive the round trip: %q", entry.Name)
	}
	if entry.Description != "Review a pull request: correctness first." {
		t.Fatalf("description did not survive the round trip: %q", entry.Description)
	}
	if entry.Body != "# Steps\n\n1. Read the tests." {
		t.Fatalf("body did not survive the round trip: %q", entry.Body)
	}
}

func TestSkillCreateAppearsInTheCatalog(t *testing.T) {
	var catalog string

	var err error

	skillCreateEnv(t)

	_, err = SkillCreateResult("deploy", "Ship the service to production.", "run the pipeline", "", false)
	if err != nil {
		t.Fatal(err)
	}

	catalog = SkillCatalog()
	if !strings.Contains(catalog, "deploy: Ship the service to production.") {
		t.Fatalf("catalog is missing the new skill: %q", catalog)
	}
}

func TestSkillCreateRejectsUnusableNames(t *testing.T) {
	var name string

	var err error

	skillCreateEnv(t)

	for _, name = range []string{"", "..", "../../etc", "has space", "UPPER/lower", strings.Repeat("a", 65)} {
		_, err = SkillCreateResult(name, "a description", "a body", "", false)
		if err == nil {
			t.Fatalf("name %q was accepted", name)
		}
	}
}

func TestSkillCreateRequiresDescriptionAndBody(t *testing.T) {
	var err error

	skillCreateEnv(t)

	_, err = SkillCreateResult("empty-description", "   ", "a body", "", false)
	if err == nil {
		t.Fatal("an empty description was accepted")
	}

	_, err = SkillCreateResult("empty-body", "a description", "  \n ", "", false)
	if err == nil {
		t.Fatal("an empty body was accepted")
	}
}

func TestSkillCreateClampsLongDescription(t *testing.T) {
	var entry *Skill

	var err error

	skillCreateEnv(t)

	_, err = SkillCreateResult("verbose", strings.Repeat("가", maxSkillDescription+50), "a body", "", false)
	if err != nil {
		t.Fatal(err)
	}

	entry = SkillFind("verbose")
	if entry == nil {
		t.Fatal("the skill did not load back")
	}

	if len([]rune(entry.Description)) != maxSkillDescription {
		t.Fatalf("description was not clamped: %d runes", len([]rune(entry.Description)))
	}
}

func TestSkillCreateRefusesToReplaceWithoutOverwrite(t *testing.T) {
	var err error

	skillCreateEnv(t)

	_, err = SkillCreateResult("notes", "Take notes.", "first body", "", false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = SkillCreateResult("notes", "Take notes differently.", "second body", "", false)
	if err == nil {
		t.Fatal("an existing skill was replaced without overwrite")
	}
	if !strings.Contains(err.Error(), ScopeProject) {
		t.Fatalf("the refusal does not name the owning scope: %v", err)
	}

	_, err = SkillCreateResult("notes", "Take notes differently.", "second body", "", true)
	if err != nil {
		t.Fatal(err)
	}

	if SkillFind("notes").Description != "Take notes differently." {
		t.Fatal("overwrite did not replace the skill")
	}
}

func TestSkillCreateRefusesToCrossScopes(t *testing.T) {
	var err error

	skillCreateEnv(t)

	_, err = SkillCreateResult("shared", "A user scoped skill.", "a body", ScopeUser, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = SkillCreateResult("shared", "A project scoped skill.", "a body", ScopeProject, true)
	if err == nil {
		t.Fatal("a user skill was replaced from the project scope")
	}
}

func TestSkillCreateRejectsUnknownScope(t *testing.T) {
	var err error

	skillCreateEnv(t)

	_, err = SkillCreateResult("somewhere", "A description.", "a body", "builtin", false)
	if err == nil {
		t.Fatal("an unknown scope was accepted")
	}
}

func TestSkillCreateStopsAtTheSkillLimit(t *testing.T) {
	var index int
	var root string

	var err error

	skillCreateEnv(t)

	root = util.Path(SKILL_DIR)

	err = os.MkdirAll(root, 0700)
	if err != nil {
		t.Fatal(err)
	}

	for index = 0; index < maxSkills; index++ {
		writeSkill(t, root, fmt.Sprintf("filler-%02d", index), skillDoc(fmt.Sprintf("filler-%02d", index), "filler", "body"))
	}

	err = SkillInit()
	if err != nil {
		t.Fatal(err)
	}

	_, err = SkillCreateResult("one-too-many", "A description.", "a body", "", false)
	if err == nil {
		t.Fatal("the skill limit was not enforced")
	}
}

func TestSkillCreateToolRejectsInvalidArguments(t *testing.T) {
	var def Def

	var err error

	skillCreateEnv(t)

	def = SkillCreate()

	if def.Permission != PermissionPrivileged {
		t.Fatalf("skill_create must stay privileged, got %v", def.Permission)
	}

	_, err = def.Execute(context.Background(), "not json")
	if err == nil {
		t.Fatal("malformed arguments were accepted")
	}

	_, err = def.Execute(context.Background(), `{"name":"ok-name","description":"A description.","body":"a body"}`)
	if err != nil {
		t.Fatal(err)
	}

	if SkillFind("ok-name") == nil {
		t.Fatal("the tool did not create the skill")
	}
}
