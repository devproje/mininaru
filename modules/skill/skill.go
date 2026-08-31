// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/goccy/go-yaml"
)

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Path        string `json:"path"`
	Scope       string `json:"scope"`
}

type skillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

const skillDirName = "skills"

const skillFile = "SKILL.md"

const (
	ScopeProject = "project"
	ScopeUser    = "user"
)

const maxSkills = 64

const maxSkillBody = 65536

const maxSkillDescription = 200

const maxCatalogChars = 4096

const catalogHeader = "# Skills\n\n" +
	`Each line below names a skill available on this machine and summarizes it in
one line — the summary is not the instructions. Call the skill tool with a
matching name to load the full instructions before following them; never
infer a skill's contents from its summary.

After finishing real work, if you used or discovered a reusable multi-step
technique (not a fact or preference — that's memory_save's job), call
skill_create to write it down for your future self. Overwrite an existing
skill of the same name if you're refining something already captured rather
than creating a near-duplicate.

`

var skillNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func splitFrontmatter(text string) (string, string, error) {
	var lines []string
	var index int

	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.ReplaceAll(text, "\r\n", "\n")

	if !strings.HasPrefix(text, "---\n") {
		return "", "", fmt.Errorf("missing yaml frontmatter")
	}

	lines = strings.Split(text, "\n")

	for index = 1; index < len(lines); index++ {
		if strings.TrimRight(lines[index], " \t") != "---" && strings.TrimRight(lines[index], " \t") != "..." {
			continue
		}

		return strings.Join(lines[1:index], "\n"), strings.TrimSpace(strings.Join(lines[index+1:], "\n")), nil
	}

	return "", "", fmt.Errorf("unterminated yaml frontmatter")
}

func skillDescription(value string) string {
	var runes []rune

	value = strings.Join(strings.Fields(value), " ")

	runes = []rune(value)
	if len(runes) <= maxSkillDescription {
		return value
	}

	return string(runes[:maxSkillDescription])
}

func skillName(meta *skillMeta, dir string) string {
	var declared string

	declared = strings.TrimSpace(meta.Name)
	if declared == "" {
		return dir
	}

	if util.SafeSegment(declared) != nil || !skillNamePattern.MatchString(declared) {
		util.Log.Warn("skill declares an unusable name, using the directory name", "dir", dir, "declared", declared)
		return dir
	}

	return declared
}

func skillParse(dir, scope string) (*Skill, error) {
	var buf []byte
	var front string
	var body string
	var meta skillMeta
	var current Skill

	var err error

	buf, err = os.ReadFile(filepath.Join(dir, skillFile))
	if err != nil {
		return nil, err
	}

	if len(buf) > maxSkillBody {
		buf = append(buf[:maxSkillBody:maxSkillBody], []byte("\n[truncated]")...)
	}

	front, body, err = splitFrontmatter(string(buf))
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(front), &meta)
	if err != nil {
		return nil, fmt.Errorf("invalid frontmatter: %w", err)
	}

	if strings.TrimSpace(meta.Description) == "" {
		return nil, fmt.Errorf("frontmatter has no description")
	}

	current = Skill{
		Name:        skillName(&meta, filepath.Base(dir)),
		Description: skillDescription(meta.Description),
		Body:        body,
		Path:        dir,
		Scope:       scope,
	}

	return &current, nil
}

func skillScan(root, scope string, seen map[string]bool, accepted []Skill) []Skill {
	var entries []os.DirEntry
	var entry os.DirEntry
	var bundle string
	var info os.FileInfo
	var current *Skill

	var err error

	entries, err = os.ReadDir(root)
	if err != nil {
		return accepted
	}

	sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })

	for _, entry = range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if util.SafeSegment(entry.Name()) != nil {
			continue
		}

		bundle = filepath.Join(root, entry.Name())

		info, err = os.Stat(bundle)
		if err != nil || !info.IsDir() {
			continue
		}

		_, err = os.Stat(filepath.Join(bundle, skillFile))
		if err != nil {
			continue
		}

		current, err = skillParse(bundle, scope)
		if err != nil {
			util.Log.Warn("ignoring an unparsable skill", "skill", entry.Name(), "root", root, "error", err)
			continue
		}

		if seen[current.Name] {
			util.Log.Warn("ignoring a duplicate skill", "skill", current.Name, "bundle", bundle)
			continue
		}

		if len(accepted) >= maxSkills {
			util.Log.Warn("ignoring skills past the limit", "limit", maxSkills, "root", root)
			return accepted
		}

		seen[current.Name] = true
		accepted = append(accepted, *current)
	}

	return accepted
}

func userSkillRoot() string {
	var home string

	var err error

	home, err = os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".mininaru", skillDirName)
}

func projectSkillRoot() string {
	return util.Path(skillDirName)
}

func skillRoots() [][2]string {
	var roots [][2]string
	var user string

	roots = append(roots, [2]string{projectSkillRoot(), ScopeProject})

	user = userSkillRoot()
	if user != "" && user != roots[0][0] {
		roots = append(roots, [2]string{user, ScopeUser})
	}

	return roots
}

func scanAll() []Skill {
	var seen map[string]bool
	var accepted []Skill
	var root [2]string

	seen = make(map[string]bool)

	for _, root = range skillRoots() {
		accepted = skillScan(root[0], root[1], seen, accepted)
	}

	return accepted
}

func CreateRoot(scope string) (string, string, error) {
	switch scope {
	case "", ScopeProject:
		return projectSkillRoot(), ScopeProject, nil
	case ScopeUser:
		if userSkillRoot() == "" {
			return "", "", fmt.Errorf("user skill root is unavailable")
		}

		return userSkillRoot(), ScopeUser, nil
	}

	return "", "", fmt.Errorf("invalid scope %q, expected %q or %q", scope, ScopeProject, ScopeUser)
}

func All() []Skill {
	return scanAll()
}

func Find(name string) *Skill {
	var current Skill
	var all []Skill

	all = scanAll()
	for _, current = range all {
		if current.Name == name {
			return &current
		}
	}

	return nil
}

func Names() []string {
	var current Skill
	var names []string

	for _, current = range scanAll() {
		names = append(names, current.Name)
	}

	return names
}

func Catalog() string {
	var current Skill
	var line string
	var lines strings.Builder

	for _, current = range scanAll() {
		line = current.Name + ": " + current.Description + "\n"
		if lines.Len()+len(line) > maxCatalogChars {
			break
		}

		lines.WriteString(line)
	}

	if lines.Len() == 0 {
		return ""
	}

	return catalogHeader + strings.TrimRight(lines.String(), "\n")
}
