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

type SkillNote struct {
	Id        string `json:"id"`
	Skill     string `json:"skill"`
	Note      string `json:"note"`
	SessionId string `json:"session_id"`
	Applied   bool   `json:"applied"`
	CreatedAt string `json:"created_at"`
}

const SkillNoteToolName = "skill_note"

const maxPendingNotes = 32

const maxSkillNoteChars = 600

const skillNoteOpenTag = "<mininaru-skill-notes>"

const skillNoteCloseTag = "</mininaru-skill-notes>"

const skillNoteHeader = `Observations recorded while this skill was used before, oldest first. They are
corrections learned in practice, not part of the skill's own instructions. Where
one contradicts the body above, the observation is the newer information. When
they have accumulated enough to be worth folding into the body, rewrite the
skill with skill_revise.`

func skillNoteQuery(skill string, pendingOnly bool) ([]SkillNote, error) {
	var query string
	var rows *sql.Rows
	var notes []SkillNote
	var note SkillNote
	var applied sql.NullString

	var err error

	if util.DB == nil {
		return notes, nil
	}

	query = `SELECT id, skill, note, session_id, applied_at, created_at FROM skill_notes
		WHERE skill = ?`
	if pendingOnly {
		query += " AND applied_at IS NULL"
	}
	query += " ORDER BY created_at ASC, rowid ASC;"

	rows, err = util.DB.Query(query, skill)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&note.Id, &note.Skill, &note.Note, &note.SessionId, &applied, &note.CreatedAt)
		if err != nil {
			return nil, err
		}

		note.Applied = applied.Valid

		notes = append(notes, note)
	}

	return notes, rows.Err()
}

func SkillNotesFor(skill string, pendingOnly bool) ([]SkillNote, error) {
	return skillNoteQuery(skill, pendingOnly)
}

func skillNoteBlock(skill string) string {
	var notes []SkillNote
	var note SkillNote
	var builder strings.Builder
	var dropped int

	var err error

	notes, err = skillNoteQuery(skill, true)
	if err != nil {
		util.Log.Warn("reading skill notes failed", "skill", skill, "error", err)
		return ""
	}

	if len(notes) == 0 {
		return ""
	}

	if len(notes) > maxPendingNotes {
		dropped = len(notes) - maxPendingNotes
		notes = notes[dropped:]
	}

	builder.WriteString(skillNoteOpenTag + "\n")
	builder.WriteString(skillNoteHeader + "\n\n")

	if dropped > 0 {
		builder.WriteString(fmt.Sprintf("(%d older observations are not shown)\n", dropped))
	}

	for _, note = range notes {
		builder.WriteString("- (" + note.CreatedAt + ") " + note.Note + "\n")
	}

	builder.WriteString(skillNoteCloseTag)

	return builder.String()
}

func SkillNoteAdd(skill, note, sessionId string) (*SkillNote, error) {
	var entry *Skill
	var record SkillNote

	var err error

	skill = strings.TrimSpace(skill)
	note = strings.Join(strings.Fields(note), " ")

	if skill == "" {
		return nil, fmt.Errorf("skill is required")
	}

	if note == "" {
		return nil, fmt.Errorf("note is required")
	}

	if len([]rune(note)) > maxSkillNoteChars {
		return nil, fmt.Errorf("note cannot exceed %d characters, record the lesson not the transcript", maxSkillNoteChars)
	}

	entry = SkillFind(skill)
	if entry == nil {
		return nil, fmt.Errorf("unknown skill %q, available: %s", skill, strings.Join(SkillNames(), ", "))
	}

	if util.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	record = SkillNote{Id: uuid.NewString(), Skill: skill, Note: note, SessionId: sessionId}

	_, err = util.DB.Exec(`INSERT INTO skill_notes (id, skill, note, session_id) VALUES (?, ?, ?, ?);`,
		record.Id, record.Skill, record.Note, record.SessionId)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func skillNoteApply(skill string) (int64, error) {
	var result sql.Result

	var err error

	if util.DB == nil {
		return 0, nil
	}

	result, err = util.DB.Exec(
		`UPDATE skill_notes SET applied_at = CURRENT_TIMESTAMP WHERE skill = ? AND applied_at IS NULL;`, skill)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func SkillNoteTool() Def {
	return Def{
		Name: SkillNoteToolName,
		Description: "Record one lesson about a skill you just used: what it failed to cover, what it got wrong, " +
			"or a step it should have named. The note is shown the next time the skill is loaded, so write it for " +
			"whoever runs the skill next, not as a summary of this conversation.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill": map[string]any{"type": "string", "description": "Name of the skill the lesson is about."},
				"note": map[string]any{"type": "string",
					"description": "One concrete, reusable lesson. State the situation and what to do about it."},
			},
			"required":             []string{"skill", "note"},
			"additionalProperties": false,
		},
		Permission: PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Skill string `json:"skill"`
				Note  string `json:"note"`
			}
			var record *SkillNote
			var pending []SkillNote

			var err error

			err = ctx.Err()
			if err != nil {
				return "", err
			}

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			record, err = SkillNoteAdd(payload.Skill, payload.Note, SessionFrom(ctx))
			if err != nil {
				return "", err
			}

			pending, err = skillNoteQuery(record.Skill, true)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("recorded a note on skill %q, %d now pending.\n\n%s",
				record.Skill, len(pending),
				"Pending notes are shown whenever the skill is loaded. Fold them into the body with skill_revise "+
					"once they are worth keeping permanently."), nil
		},
	}
}
