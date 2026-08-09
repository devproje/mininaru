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
)

func writeSkill(t *testing.T, root, dir, content string) string {
	var bundle string

	var err error

	t.Helper()

	bundle = filepath.Join(root, dir)

	err = os.MkdirAll(bundle, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(bundle, SKILL_FILE), []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}

	return bundle
}

func skillDoc(name, description, body string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s\n", name, description, body)
}

func TestSkillLoadsEcosystemFrontmatter(t *testing.T) {
	var root string
	var entry *Skill

	var err error

	root = t.TempDir()

	writeSkill(t, root, "review", `---
name: review
description: How to review a pull request in this repository.
allowed-tools: Read, Grep, Bash(git diff:*)
license: Apache-2.0
metadata:
  author: someone
  tags:
    - review
    - git
---

Read the diff first, then the tests.
`)

	err = SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	entry = SkillFind("review")
	if entry == nil {
		t.Fatalf("a bundle with extra frontmatter keys was rejected: %#v", SkillAll())
	}
	if entry.Description != "How to review a pull request in this repository." {
		t.Fatalf("description = %q", entry.Description)
	}
	if !strings.Contains(entry.Body, "Read the diff first") {
		t.Fatalf("body = %q", entry.Body)
	}
	if entry.Scope != ScopeProject {
		t.Fatalf("scope = %q", entry.Scope)
	}
}

func TestSkillSkipsBrokenBundles(t *testing.T) {
	var root string

	var err error

	root = t.TempDir()

	writeSkill(t, root, "good", skillDoc("good", "a usable skill", "body"))
	writeSkill(t, root, "no-front", "just a markdown file\n")
	writeSkill(t, root, "unterminated", "---\nname: x\ndescription: y\n")
	writeSkill(t, root, "broken-yaml", "---\nname: [unclosed\n---\nbody\n")
	writeSkill(t, root, "no-desc", "---\nname: no-desc\n---\nbody\n")

	err = os.MkdirAll(filepath.Join(root, "not-a-skill"), 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(SkillAll()) != 1 || SkillAll()[0].Name != "good" {
		t.Fatalf("broken bundles were not skipped: %#v", SkillNames())
	}
}

func TestSkillProjectBeatsUser(t *testing.T) {
	var project string
	var user string

	var err error

	project = t.TempDir()
	user = t.TempDir()

	writeSkill(t, project, "shared", skillDoc("shared", "the project copy", "project body"))
	writeSkill(t, user, "shared", skillDoc("shared", "the user copy", "user body"))
	writeSkill(t, user, "only-user", skillDoc("only-user", "only in the user root", "body"))

	err = SkillInitAt(project, user)
	if err != nil {
		t.Fatal(err)
	}

	if len(SkillAll()) != 2 {
		t.Fatalf("skills = %#v", SkillNames())
	}
	if SkillFind("shared").Description != "the project copy" {
		t.Fatal("the user copy won a name collision")
	}
	if SkillFind("only-user").Scope != ScopeUser {
		t.Fatal("a user-only skill lost its scope")
	}
}

func TestSkillDeduplicatesIdenticalRoots(t *testing.T) {
	var root string

	var err error

	root = t.TempDir()

	writeSkill(t, root, "once", skillDoc("once", "should appear a single time", "body"))

	err = SkillInitAt(root, root)
	if err != nil {
		t.Fatal(err)
	}

	if len(SkillAll()) != 1 {
		t.Fatalf("the same root was scanned twice: %#v", SkillNames())
	}
}

func TestSkillRejectsUnusableDeclaredName(t *testing.T) {
	var root string

	var err error

	root = t.TempDir()

	writeSkill(t, root, "safe-dir", skillDoc("../evil", "a hostile declared name", "body"))

	err = SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	if SkillFind("safe-dir") == nil {
		t.Fatalf("the directory-name fallback did not apply: %#v", SkillNames())
	}
	if SkillFind("../evil") != nil {
		t.Fatal("a traversal name became a lookup key")
	}
}

func TestSkillCatalogStaysBounded(t *testing.T) {
	var root string
	var index int

	var err error

	root = t.TempDir()

	for index = 0; index < 100; index++ {
		writeSkill(t, root, fmt.Sprintf("skill-%03d", index),
			skillDoc(fmt.Sprintf("skill-%03d", index), strings.Repeat("long description ", 40), "body"))
	}

	err = SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(SkillAll()) > maxSkills {
		t.Fatalf("accepted %d skills", len(SkillAll()))
	}
	if len(SkillCatalog()) > maxCatalogChars {
		t.Fatalf("catalog is %d chars", len(SkillCatalog()))
	}
	if len([]rune(SkillAll()[0].Description)) > maxSkillDescription {
		t.Fatalf("description is %d runes", len([]rune(SkillAll()[0].Description)))
	}
}

func TestSkillToolReturnsBodyAndBundlePath(t *testing.T) {
	var root string
	var bundle string
	var def Def
	var output string

	var err error

	root = t.TempDir()
	bundle = writeSkill(t, root, "deploy", skillDoc("deploy", "how to deploy", "Run the script."))

	err = os.WriteFile(filepath.Join(bundle, "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	def = SkillLoad()

	output, err = def.Execute(context.Background(), `{"name":"deploy"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Run the script.") {
		t.Fatalf("body missing: %q", output)
	}
	if !strings.Contains(output, "run.sh") || !strings.Contains(output, "bash_exec") {
		t.Fatalf("companion listing missing: %q", output)
	}
	if !strings.Contains(output, bundle) {
		t.Fatalf("bundle path missing: %q", output)
	}

	output, err = def.Execute(context.Background(), `{"name":"deploy","path":"run.sh"}`)
	if err != nil || !strings.Contains(output, "echo hi") {
		t.Fatalf("companion read = %q, %v", output, err)
	}

	_, err = def.Execute(context.Background(), `{"name":"nope"}`)
	if err == nil || !strings.Contains(err.Error(), "deploy") {
		t.Fatalf("unknown skill error = %v", err)
	}
}

func TestSkillToolSandboxesCompanionReads(t *testing.T) {
	var root string
	var outside string
	var bundle string
	var def Def

	var err error

	root = t.TempDir()
	outside = t.TempDir()

	bundle = writeSkill(t, root, "leaky", skillDoc("leaky", "tries to escape", "body"))

	err = os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(bundle, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}

	err = SkillInitAt(root, "")
	if err != nil {
		t.Fatal(err)
	}

	def = SkillLoad()

	_, err = def.Execute(context.Background(), `{"name":"leaky","path":"../../etc/passwd"}`)
	if err == nil {
		t.Fatal("path traversal was allowed")
	}

	_, err = def.Execute(context.Background(), `{"name":"leaky","path":"link.txt"}`)
	if err == nil {
		t.Fatal("a symlink out of the bundle was followed")
	}

	_, err = def.Execute(context.Background(), `{"name":"leaky","path":".git/config"}`)
	if err == nil {
		t.Fatal("a hidden path was allowed")
	}
}
