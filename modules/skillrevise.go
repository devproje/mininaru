// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type SkillRevision struct {
	Id          string `json:"id"`
	Skill       string `json:"skill"`
	Scope       string `json:"scope"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}

const SkillReviseToolName = "skill_revise"

const maxSkillRevisions = 20

func skillRevisionSave(entry *Skill, reason string) (string, error) {
	var id string

	var err error

	if util.DB == nil {
		return "", fmt.Errorf("database is not initialized")
	}

	id = uuid.NewString()

	_, err = util.DB.Exec(
		`INSERT INTO skill_revisions (id, skill, scope, path, description, body, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?);`,
		id, entry.Name, entry.Scope, entry.Path, entry.Description, entry.Body, reason)
	if err != nil {
		return "", err
	}

	_, err = util.DB.Exec(
		`DELETE FROM skill_revisions WHERE skill = ? AND id NOT IN (
			SELECT id FROM skill_revisions WHERE skill = ? ORDER BY created_at DESC, rowid DESC LIMIT ?
		);`, entry.Name, entry.Name, maxSkillRevisions)
	if err != nil {
		util.Log.Warn("trimming skill revisions failed", "skill", entry.Name, "error", err)
	}

	return id, nil
}

func SkillRevisions(skill string) ([]SkillRevision, error) {
	var rows *sql.Rows
	var revisions []SkillRevision
	var revision SkillRevision

	var err error

	if util.DB == nil {
		return revisions, nil
	}

	rows, err = util.DB.Query(
		`SELECT id, skill, scope, path, description, body, reason, created_at FROM skill_revisions
		WHERE skill = ? ORDER BY created_at DESC, rowid DESC;`, skill)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&revision.Id, &revision.Skill, &revision.Scope, &revision.Path,
			&revision.Description, &revision.Body, &revision.Reason, &revision.CreatedAt)
		if err != nil {
			return nil, err
		}

		revisions = append(revisions, revision)
	}

	return revisions, rows.Err()
}

func SkillReviseResult(name, description, body, reason string) (string, error) {
	var entry *Skill
	var root string
	var scope string
	var document []byte
	var revisionId string
	var target string
	var applied int64

	var err error

	name = strings.TrimSpace(name)
	body = strings.TrimSpace(body)
	reason = strings.Join(strings.Fields(reason), " ")

	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	if body == "" {
		return "", fmt.Errorf("body is required")
	}

	if len(body) > maxSkillBody {
		return "", fmt.Errorf("body cannot exceed %d bytes", maxSkillBody)
	}

	entry = SkillFind(name)
	if entry == nil {
		return "", fmt.Errorf("unknown skill %q, available: %s", name, strings.Join(SkillNames(), ", "))
	}

	description = skillDescription(description)
	if description == "" {
		description = entry.Description
	}

	root, scope, err = skillCreateRoot(entry.Scope)
	if err != nil {
		return "", err
	}

	revisionId, err = skillRevisionSave(entry, reason)
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

	applied, err = skillNoteApply(name)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"skill: %s\nscope: %s\npath: %s\nrevision: %s\nnotes folded in: %d\n\n%s",
		name, scope, target, revisionId, applied,
		"The previous version is kept in the revision history and can be restored with "+
			"`mininaru skill rollback`. The pending notes are now marked as applied."), nil
}

func SkillRestore(name, revisionId string) (string, error) {
	var revisions []SkillRevision
	var revision SkillRevision
	var target SkillRevision
	var found bool
	var entry *Skill
	var root string
	var document []byte
	var written string

	var err error

	revisions, err = SkillRevisions(name)
	if err != nil {
		return "", err
	}

	if len(revisions) == 0 {
		return "", fmt.Errorf("skill %q has no revision history", name)
	}

	if revisionId == "" {
		target = revisions[0]
		found = true
	}

	for _, revision = range revisions {
		if found {
			break
		}
		if revision.Id != revisionId {
			continue
		}

		target = revision
		found = true
	}

	if !found {
		return "", fmt.Errorf("skill %q has no revision %q", name, revisionId)
	}

	entry = SkillFind(name)
	if entry != nil {
		_, err = skillRevisionSave(entry, "superseded by a rollback to "+target.Id)
		if err != nil {
			return "", err
		}

		root, _, err = skillCreateRoot(entry.Scope)
	} else {
		root, _, err = skillCreateRoot(target.Scope)
	}
	if err != nil {
		return "", err
	}

	document, err = skillDocument(name, target.Description, target.Body)
	if err != nil {
		return "", err
	}

	written, err = skillWrite(root, name, document)
	if err != nil {
		return "", err
	}

	err = SkillInit()
	if err != nil {
		return "", err
	}

	return written, nil
}

func SkillRevise() Def {
	return Def{
		Name: SkillReviseToolName,
		Description: "Rewrite an existing skill's instructions, folding in the observations recorded against it. " +
			"The previous version is snapshotted first so it can be restored. Use this instead of skill_create " +
			"when the skill already exists and you are improving it rather than replacing it wholesale.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Name of an existing skill."},
				"body": map[string]any{"type": "string",
					"description": "The complete new markdown instructions, without frontmatter. This replaces the body, so carry forward everything still correct."},
				"description": map[string]any{"type": "string",
					"description": "Optional replacement one-line summary. The existing description is kept when this is omitted."},
				"reason": map[string]any{"type": "string",
					"description": "One line on what this revision changes and why, stored with the snapshot of the previous version."},
			},
			"required":             []string{"name", "body"},
			"additionalProperties": false,
		},
		Permission: PermissionPrivileged,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Name        string `json:"name"`
				Body        string `json:"body"`
				Description string `json:"description"`
				Reason      string `json:"reason"`
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

			return SkillReviseResult(payload.Name, payload.Description, payload.Body, payload.Reason)
		},
	}
}
