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

type memoryEntry struct {
	Id      string `json:"id"`
	Content string `json:"content"`
}

const MemoryToolName = "memory"

const memoryMaxChars = 4096

func memoryEntries() ([]memoryEntry, error) {
	var entries []memoryEntry
	var rows *sql.Rows
	var entry memoryEntry

	var err error

	if util.DB == nil {
		return entries, nil
	}

	rows, err = util.DB.Query("SELECT id, content FROM memories ORDER BY created_at ASC, rowid ASC;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&entry.Id, &entry.Content)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

func memoryUsage(excludeId string) (int, error) {
	var used int

	var err error

	if util.DB == nil {
		return 0, fmt.Errorf("database is not initialized")
	}

	err = util.DB.QueryRow("SELECT COALESCE(SUM(length(content)), 0) FROM memories WHERE id != ?;", excludeId).Scan(&used)
	return used, err
}

func MemorySnapshot() string {
	var entries []memoryEntry
	var entry memoryEntry
	var lines []string

	var err error

	entries, err = memoryEntries()
	if err != nil || len(entries) == 0 {
		return ""
	}

	for _, entry = range entries {
		lines = append(lines, "- "+entry.Content)
	}

	return strings.Join(lines, "\n")
}

func memoryResult() (string, error) {
	var entries []memoryEntry
	var buf []byte

	var err error

	entries, err = memoryEntries()
	if err != nil {
		return "", err
	}
	buf, err = json.Marshal(map[string]any{"entries": entries, "limit_chars": memoryMaxChars})
	return string(buf), err
}

func Memory() Def {
	return Def{
		Name:        MemoryToolName,
		Description: "Manage durable memories shared between the CLI and the paired Discord owner. Save stable user preferences, facts, and decisions; do not save secrets or temporary details.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":  map[string]any{"type": "string", "enum": []string{"list", "add", "replace", "remove"}},
				"id":      map[string]any{"type": "string", "description": "Entry id required by replace and remove."},
				"content": map[string]any{"type": "string", "description": "Compact durable fact required by add and replace."},
			},
			"required":             []string{"action"},
			"additionalProperties": false,
		},
		Permission: PermissionPrivileged,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Action  string `json:"action"`
				Id      string `json:"id"`
				Content string `json:"content"`
			}
			var used int
			var result sql.Result
			var affected int64

			var err error

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			payload.Content = strings.TrimSpace(payload.Content)

			switch payload.Action {
			case "list":
				return memoryResult()
			case "add":
				if payload.Content == "" {
					return "", fmt.Errorf("content is required")
				}
				used, err = memoryUsage("")
				if err != nil {
					return "", err
				}
				if used+len([]rune(payload.Content)) > memoryMaxChars {
					return "", fmt.Errorf("memory limit of %d characters would be exceeded", memoryMaxChars)
				}
				_, err = util.DB.Exec("INSERT OR IGNORE INTO memories (id, content) VALUES (?, ?);", uuid.NewString(), payload.Content)
			case "replace":
				if payload.Id == "" || payload.Content == "" {
					return "", fmt.Errorf("id and content are required")
				}
				used, err = memoryUsage(payload.Id)
				if err != nil {
					return "", err
				}
				if used+len([]rune(payload.Content)) > memoryMaxChars {
					return "", fmt.Errorf("memory limit of %d characters would be exceeded", memoryMaxChars)
				}
				result, err = util.DB.Exec("UPDATE memories SET content = ? WHERE id = ?;", payload.Content, payload.Id)
			case "remove":
				if payload.Id == "" {
					return "", fmt.Errorf("id is required")
				}
				result, err = util.DB.Exec("DELETE FROM memories WHERE id = ?;", payload.Id)
			default:
				return "", fmt.Errorf("invalid action %q", payload.Action)
			}
			if err != nil {
				return "", err
			}
			if result != nil {
				affected, err = result.RowsAffected()
				if err != nil {
					return "", err
				}
				if affected == 0 {
					return "", fmt.Errorf("memory id %q not found", payload.Id)
				}
			}

			return memoryResult()
		},
	}
}
