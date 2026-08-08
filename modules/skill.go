package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/devproje/mininaru/util"
	"go.yaml.in/yaml/v3"
)

type Skill struct {
	Name        string
	Description string
	Body        string
	Path        string
	Scope       string
}

type skillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

const SKILL_DIR = "skills"

const SKILL_FILE = "SKILL.md"

const (
	ScopeProject = "project"
	ScopeUser    = "user"
)

const maxSkills = 64

const maxSkillBody = 65536

const maxSkillDescription = 200

const maxCatalogChars = 4096

var skills []Skill

var skillMu sync.RWMutex

var skillNamePattern *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

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

	if declared != dir {
		util.Log.Debug("skill declares a name different from its directory", "dir", dir, "declared", declared)
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

	buf, err = os.ReadFile(filepath.Join(dir, SKILL_FILE))
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
		util.Log.Warn("cannot read a skill root", "root", root, "error", err)
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

		_, err = os.Stat(filepath.Join(bundle, SKILL_FILE))
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

func skillRoots(project, user string) [][2]string {
	var resolved string
	var roots [][2]string

	var err error

	resolved, err = toolRoot(project)
	if err == nil {
		roots = append(roots, [2]string{resolved, ScopeProject})
	}

	if user == "" {
		return roots
	}

	resolved, err = toolRoot(user)
	if err != nil {
		return roots
	}

	if len(roots) > 0 && roots[0][0] == resolved {
		return roots
	}

	return append(roots, [2]string{resolved, ScopeUser})
}

func userSkillRoot() string {
	var home string

	var err error

	home, err = os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".mininaru", SKILL_DIR)
}

func SkillAll() []Skill {
	skillMu.RLock()
	defer skillMu.RUnlock()

	return skills
}

func SkillFind(name string) *Skill {
	var index int

	skillMu.RLock()
	defer skillMu.RUnlock()

	for index = range skills {
		if skills[index].Name != name {
			continue
		}

		return &skills[index]
	}

	return nil
}

func SkillNames() []string {
	var current Skill
	var names []string

	for _, current = range SkillAll() {
		names = append(names, current.Name)
	}

	return names
}

func SkillCatalog() string {
	var current Skill
	var line string
	var builder strings.Builder

	skillMu.RLock()
	defer skillMu.RUnlock()

	for _, current = range skills {
		line = current.Name + ": " + current.Description + "\n"
		if builder.Len()+len(line) > maxCatalogChars {
			break
		}

		builder.WriteString(line)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func SkillInitAt(project, user string) error {
	var seen map[string]bool
	var root [2]string
	var accepted []Skill

	seen = make(map[string]bool)

	for _, root = range skillRoots(project, user) {
		accepted = skillScan(root[0], root[1], seen, accepted)
	}

	skillMu.Lock()
	defer skillMu.Unlock()

	skills = accepted

	return nil
}

func SkillReload() error {
	return SkillInit()
}

func SkillInit() error {
	return SkillInitAt(util.Path(SKILL_DIR), userSkillRoot())
}
