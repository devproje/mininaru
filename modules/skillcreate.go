// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devproje/mininaru/util"
	"go.yaml.in/yaml/v3"
)

const SkillCreateToolName = "skill_create"

func skillCreateRoot(scope string) (string, string, error) {
	var root string

	switch scope {
	case "", ScopeProject:
		return util.Path(SKILL_DIR), ScopeProject, nil
	case ScopeUser:
		root = userSkillRoot()
		if root == "" {
			return "", "", fmt.Errorf("user skill root is unavailable")
		}

		return root, ScopeUser, nil
	}

	return "", "", fmt.Errorf("invalid scope %q, expected %q or %q", scope, ScopeProject, ScopeUser)
}

func skillCreateName(name string) error {
	var err error

	if name == "" {
		return fmt.Errorf("name is required")
	}

	err = util.SafeSegment(name)
	if err != nil {
		return err
	}

	if !skillNamePattern.MatchString(name) {
		return fmt.Errorf("name %q must match %s", name, skillNamePattern.String())
	}

	return nil
}

func skillCreateConflict(name, scope string, overwrite bool) error {
	var existing *Skill

	existing = SkillFind(name)
	if existing == nil {
		if len(SkillAll()) >= maxSkills {
			return fmt.Errorf("skill limit of %d is already reached", maxSkills)
		}

		return nil
	}

	if !overwrite {
		return fmt.Errorf("skill %q already exists in the %s scope at %s, pass overwrite to replace it",
			name, existing.Scope, existing.Path)
	}

	if existing.Scope != scope {
		return fmt.Errorf("skill %q already exists in the %s scope at %s, which the %s scope cannot replace",
			name, existing.Scope, existing.Path, scope)
	}

	return nil
}

func skillDocument(name, description, body string) ([]byte, error) {
	var front []byte
	var builder strings.Builder

	var err error

	front, err = yaml.Marshal(&skillMeta{Name: name, Description: description})
	if err != nil {
		return nil, err
	}

	builder.WriteString("---\n")
	builder.Write(front)
	builder.WriteString("---\n\n")
	builder.WriteString(body)
	builder.WriteString("\n")

	return []byte(builder.String()), nil
}

func skillWrite(root, name string, document []byte) (string, error) {
	var bundle string
	var target string

	var err error

	bundle = filepath.Join(root, name)

	err = os.MkdirAll(bundle, 0700)
	if err != nil {
		return "", err
	}

	target = filepath.Join(bundle, SKILL_FILE)

	err = util.WriteFileAtomic(target, document, 0600)
	if err != nil {
		return "", err
	}

	return target, nil
}

func SkillCreateResult(name, description, body, scope string, overwrite bool) (string, error) {
	var root string
	var document []byte
	var target string

	var err error

	name = strings.TrimSpace(name)
	description = skillDescription(description)
	body = strings.TrimSpace(body)

	err = skillCreateName(name)
	if err != nil {
		return "", err
	}

	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	if body == "" {
		return "", fmt.Errorf("body is required")
	}

	if len(body) > maxSkillBody {
		return "", fmt.Errorf("body cannot exceed %d bytes", maxSkillBody)
	}

	root, scope, err = skillCreateRoot(strings.TrimSpace(scope))
	if err != nil {
		return "", err
	}

	err = skillCreateConflict(name, scope, overwrite)
	if err != nil {
		return "", err
	}

	document, err = skillDocument(name, description, body)
	if err != nil {
		return "", err
	}

	target, err = skillWrite(root, name, document)
	if err != nil {
		return "", err
	}

	err = SkillInit()
	if err != nil {
		return "", err
	}

	if SkillFind(name) == nil {
		return "", fmt.Errorf("skill %q was written to %s but did not load back", name, target)
	}

	return fmt.Sprintf("skill: %s\nscope: %s\npath: %s\ndescription: %s\n\n%s",
		name, scope, target, description,
		"The skill is loaded. It appears in the skill catalog from the next turn onward."), nil
}

func SkillCreate() Def {
	return Def{
		Name:        SkillCreateToolName,
		Description: "Create or replace a skill bundle on disk so it becomes loadable by the skill tool. The body holds the instructions only; the frontmatter is generated.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":        map[string]any{"type": "string", "description": "Skill name in kebab-case, matching ^[a-zA-Z0-9_-]{1,64}$."},
				"description": map[string]any{"type": "string", "description": "One line summarizing when to use the skill. Truncated past 200 characters."},
				"body":        map[string]any{"type": "string", "description": "Markdown instructions, without the yaml frontmatter."},
				"scope":       map[string]any{"type": "string", "enum": []string{ScopeProject, ScopeUser}, "description": "Where to write the bundle. Defaults to project."},
				"overwrite":   map[string]any{"type": "boolean", "description": "Replace a skill of the same name in the same scope."},
			},
			"required":             []string{"name", "description", "body"},
			"additionalProperties": false,
		},
		Permission: PermissionPrivileged,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Body        string `json:"body"`
				Scope       string `json:"scope"`
				Overwrite   bool   `json:"overwrite"`
			}

			var err error

			err = ctx.Err()
			if err != nil {
				return "", err
			}

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			return SkillCreateResult(payload.Name, payload.Description, payload.Body, payload.Scope, payload.Overwrite)
		},
	}
}
