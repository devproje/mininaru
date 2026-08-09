// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
)

func pinSetup(t *testing.T) {
	var version string
	var branch string
	var hash string

	t.Helper()

	version = util.AppVersion
	branch = util.AppBranch
	hash = util.AppHash

	t.Cleanup(func() {
		util.AppVersion = version
		util.AppBranch = branch
		util.AppHash = hash
	})

	util.AppVersion = "v9.9.9"
	util.AppBranch = "release"
	util.AppHash = "deadbee"
}

func TestSystemPromptPinsRuntimeIdentity(t *testing.T) {
	var prompt string

	pinSetup(t)

	prompt = systemPrompt(&NaruAgent{Role: "you are naru", Soul: "be brief"}, nil)

	if !strings.Contains(prompt, "mininaru v9.9.9-deadbee (branch: release)") {
		t.Fatalf("system prompt is missing the host line:\n%s", prompt)
	}
	if !strings.Contains(prompt, runtimeOpenTag) || !strings.Contains(prompt, runtimeCloseTag) {
		t.Fatalf("system prompt is missing the runtime tags:\n%s", prompt)
	}
	if !strings.Contains(prompt, "you are naru") || !strings.Contains(prompt, "be brief") {
		t.Fatalf("system prompt dropped the persona:\n%s", prompt)
	}

	if strings.Index(prompt, runtimeOpenTag) > strings.Index(prompt, "you are naru") {
		t.Fatal("persona was placed before the runtime block")
	}
}

func TestSystemPromptIncludesOnlyActiveAgentIdentity(t *testing.T) {
	var prompt string

	prompt = systemPrompt(&NaruAgent{Id: "agent-123", Name: "naru", Role: "be helpful"}, nil)

	if !strings.Contains(prompt, agentOpenTag) || !strings.Contains(prompt, "id: agent-123") ||
		!strings.Contains(prompt, `name: "naru"`) {
		t.Fatalf("active agent identity is missing:\n%s", prompt)
	}
	if strings.Index(prompt, runtimeOpenTag) > strings.Index(prompt, agentOpenTag) {
		t.Fatal("agent identity was placed before the runtime block")
	}
	if strings.Index(prompt, agentOpenTag) > strings.Index(prompt, "be helpful") {
		t.Fatal("persona was placed before agent identity")
	}

	prompt = systemPrompt(nil, nil)
	if strings.Contains(prompt, agentOpenTag) {
		t.Fatalf("nil agent emitted an identity block:\n%s", prompt)
	}
}

func TestSystemPromptSurvivesEmptyPersona(t *testing.T) {
	var prompt string

	pinSetup(t)

	prompt = systemPrompt(&NaruAgent{}, nil)
	if !strings.Contains(prompt, "mininaru v9.9.9-deadbee") {
		t.Fatalf("an agent without a persona lost the host line:\n%s", prompt)
	}
	if strings.HasSuffix(prompt, "\n\n") {
		t.Fatalf("empty persona left a trailing separator: %q", prompt[len(prompt)-8:])
	}

	prompt = systemPrompt(nil, nil)
	if !strings.Contains(prompt, "mininaru v9.9.9-deadbee") {
		t.Fatalf("a nil agent lost the host line:\n%s", prompt)
	}
}

func skillSetup(t *testing.T) []modules.Def {
	var root string
	var bundle string

	var err error

	t.Helper()

	root = t.TempDir()
	bundle = filepath.Join(root, "deploy")

	err = os.MkdirAll(bundle, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(bundle, "SKILL.md"),
		[]byte("---\nname: deploy\ndescription: how to ship this repository\n---\n\nSECRET-BODY-MARKER\n"), 0600)
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

func TestSystemPromptAdvertisesSkills(t *testing.T) {
	var defs []modules.Def
	var prompt string

	pinSetup(t)
	defs = skillSetup(t)

	prompt = systemPrompt(&NaruAgent{Role: "you are naru"}, defs)

	if !strings.Contains(prompt, skillOpenTag) || !strings.Contains(prompt, "deploy: how to ship this repository") {
		t.Fatalf("catalog missing:\n%s", prompt)
	}
	if strings.Contains(prompt, "SECRET-BODY-MARKER") {
		t.Fatalf("the skill body leaked into the system prompt:\n%s", prompt)
	}

	if strings.Index(prompt, runtimeOpenTag) > strings.Index(prompt, skillOpenTag) {
		t.Fatal("the catalog was placed before the runtime block")
	}
	if strings.Index(prompt, skillOpenTag) > strings.Index(prompt, "you are naru") {
		t.Fatal("the persona was placed before the catalog")
	}
}

func TestSystemPromptHidesSkillsWithoutTheTool(t *testing.T) {
	var prompt string

	pinSetup(t)
	skillSetup(t)

	prompt = systemPrompt(&NaruAgent{Role: "you are naru"}, nil)
	if strings.Contains(prompt, skillOpenTag) {
		t.Fatalf("skills were advertised without the tool that loads them:\n%s", prompt)
	}

	prompt = systemPrompt(&NaruAgent{Role: "you are naru"}, []modules.Def{modules.CurrentTime()})
	if strings.Contains(prompt, skillOpenTag) {
		t.Fatalf("an unrelated tool set advertised skills:\n%s", prompt)
	}
}

func TestSystemPromptOmitsEmptyCatalog(t *testing.T) {
	var prompt string

	var err error

	pinSetup(t)

	err = modules.SkillInitAt(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	prompt = systemPrompt(&NaruAgent{Role: "you are naru"}, []modules.Def{modules.SkillLoad()})
	if strings.Contains(prompt, skillOpenTag) {
		t.Fatalf("an empty catalog still emitted a block:\n%s", prompt)
	}
	if strings.Contains(prompt, "\n\n\n") {
		t.Fatalf("an empty catalog left a blank gap:\n%s", prompt)
	}
}

func TestSystemPromptStatesPrecedence(t *testing.T) {
	var prompt string

	pinSetup(t)

	prompt = systemPrompt(&NaruAgent{Role: "you are naru"}, nil)

	if !strings.Contains(prompt, "outranks") {
		t.Fatal("the runtime block does not claim precedence over the persona")
	}
	if !strings.Contains(prompt, "mistaken") {
		t.Fatal("the runtime block does not tell the model to reject user corrections")
	}
}

func TestSystemPromptShowsMemoryOnlyWithPrivilegedTool(t *testing.T) {
	var prompt string

	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = util.DB.Exec("INSERT INTO memories (id, content) VALUES ('one', 'User prefers concise Korean replies');")
	if err != nil {
		t.Fatal(err)
	}

	prompt = systemPrompt(&NaruAgent{Role: "you are naru"}, []modules.Def{modules.Memory()})
	if !strings.Contains(prompt, memoryOpenTag) || !strings.Contains(prompt, "User prefers concise Korean replies") {
		t.Fatalf("memory missing for privileged caller:\n%s", prompt)
	}

	prompt = systemPrompt(&NaruAgent{Role: "you are naru"}, modules.SafeTools())
	if strings.Contains(prompt, memoryOpenTag) || strings.Contains(prompt, "User prefers concise Korean replies") {
		t.Fatalf("memory leaked to an unprivileged caller:\n%s", prompt)
	}
}

func TestSystemPromptAdvertisesEmptyMemoryStore(t *testing.T) {
	var prompt string

	var err error

	util.DB, err = util.InitDatabase(filepath.Join(t.TempDir(), "empty-memory.db"))
	if err != nil {
		t.Fatal(err)
	}

	prompt = systemPrompt(&NaruAgent{}, []modules.Def{modules.Memory()})
	if !strings.Contains(prompt, memoryOpenTag) || !strings.Contains(prompt, "(empty)") ||
		!strings.Contains(prompt, "Use the memory tool proactively") {
		t.Fatalf("empty memory capability was not advertised:\n%s", prompt)
	}
}
