// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
)

const indexFile = "MEMORY.md"

const indexMaxLines = 200
const indexMaxBytes = 25 * 1024

const indexHeader = "# Persistent memory\n\n"

const frontmatterDelim = "---"

type memoryMetadata struct {
	Type string `yaml:"type"`
}

type memoryFrontmatter struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Metadata    memoryMetadata `yaml:"metadata"`
	Modified    string         `yaml:"modified,omitempty"`
}

var memoryTypes = map[string]bool{
	"user":      true,
	"feedback":  true,
	"project":   true,
	"reference": true,
}

var indexLinePattern = regexp.MustCompile(`(?m)^-\s\[[^\]]*\]\(([^)]+)\)[^\n]*\n?`)

var writeMu sync.Mutex

func memoryDir(agentId string) (string, error) {
	var err error

	err = util.SafeSegment(agentId)
	if err != nil {
		return "", err
	}

	return util.Path(filepath.Join("memory", agentId)), nil
}

func slugToFile(name string) (string, error) {
	var err error

	if !strings.HasSuffix(name, ".md") {
		name = name + ".md"
	}

	err = util.SafeSegment(name)
	if err != nil {
		return "", err
	}

	return name, nil
}

func indexPath(dir string) string {
	return filepath.Join(dir, indexFile)
}

func topicPath(dir, file string) string {
	return filepath.Join(dir, file)
}

func readIndex(dir string) (string, error) {
	var buf []byte

	var err error

	buf, err = os.ReadFile(indexPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}

		return "", err
	}

	return string(buf), nil
}

func upsertIndexLine(dir, file, name, description string) error {
	var current string
	var line string
	var found bool

	var err error

	current, err = readIndex(dir)
	if err != nil {
		return err
	}

	line = fmt.Sprintf("- [%s](%s) — %s\n", name, file, description)

	found = false
	current = indexLinePattern.ReplaceAllStringFunc(current, func(match string) string {
		var loc []string

		loc = indexLinePattern.FindStringSubmatch(match)
		if len(loc) == 2 && loc[1] == file {
			found = true
			return line
		}

		return match
	})

	if !found {
		if current != "" && !strings.HasSuffix(current, "\n") {
			current += "\n"
		}
		current += line
	}

	return util.WriteFileAtomic(indexPath(dir), []byte(current), 0600)
}

func removeIndexLine(dir, file string) error {
	var current string
	var updated string

	var err error

	current, err = readIndex(dir)
	if err != nil {
		return err
	}

	updated = indexLinePattern.ReplaceAllStringFunc(current, func(match string) string {
		var loc []string

		loc = indexLinePattern.FindStringSubmatch(match)
		if len(loc) == 2 && loc[1] == file {
			return ""
		}

		return match
	})

	return util.WriteFileAtomic(indexPath(dir), []byte(updated), 0600)
}

func indexOverLimit(dir string) (bool, error) {
	var current string
	var lines []string

	var err error

	current, err = readIndex(dir)
	if err != nil {
		return false, err
	}

	if len(current) > indexMaxBytes {
		return true, nil
	}

	lines = strings.Split(current, "\n")
	return len(lines) > indexMaxLines, nil
}

func writeTopic(dir, file, name, description, memType, content string) error {
	var front memoryFrontmatter
	var encoded []byte
	var buf bytes.Buffer

	var err error

	front = memoryFrontmatter{
		Name:        name,
		Description: description,
		Metadata:    memoryMetadata{Type: memType},
		Modified:    time.Now().UTC().Format(time.RFC3339),
	}

	encoded, err = yaml.Marshal(front)
	if err != nil {
		return err
	}

	buf.WriteString(frontmatterDelim + "\n")
	buf.Write(encoded)
	buf.WriteString(frontmatterDelim + "\n\n")
	buf.WriteString(content)

	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(topicPath(dir, file), buf.Bytes(), 0600)
}

func capIndex(content string) string {
	var lines []string
	var truncated bool

	if len(content) > indexMaxBytes {
		content = content[:indexMaxBytes]
		truncated = true
	}

	lines = strings.Split(content, "\n")
	if len(lines) > indexMaxLines {
		lines = lines[:indexMaxLines]
		content = strings.Join(lines, "\n")
		truncated = true
	}

	if truncated {
		content += "\n[memory index truncated]"
	}

	return content
}

func saveTool(agentId string) modules.Tool {
	return modules.Tool{
		Name: "memory_save",
		Description: "Save or update a persistent memory note for this agent. Notes persist across every future " +
			"session with this agent and are summarized in the memory index shown at the start of each conversation. " +
			"Use this for facts, preferences, or corrections that aren't derivable from the code or conversation history.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":        map[string]any{"type": "string", "description": "short kebab-case slug identifying this memory"},
				"description": map[string]any{"type": "string", "description": "one-line summary shown in the memory index"},
				"type": map[string]any{
					"type": "string",
					"enum": []string{"user", "feedback", "project", "reference"},
				},
				"content": map[string]any{"type": "string", "description": "full memory content, written as markdown"},
			},
			"required":             []string{"name", "description", "type", "content"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Type        string `json:"type"`
				Content     string `json:"content"`
			}
			var dir string
			var file string
			var over bool
			var result string

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}
			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Name == "" {
				return "", fmt.Errorf("name is required")
			}
			if payload.Description == "" {
				return "", fmt.Errorf("description is required")
			}
			if !memoryTypes[payload.Type] {
				return "", fmt.Errorf("type must be one of user, feedback, project, reference")
			}
			if payload.Content == "" {
				return "", fmt.Errorf("content is required")
			}

			dir, err = memoryDir(agentId)
			if err != nil {
				return "", err
			}
			file, err = slugToFile(payload.Name)
			if err != nil {
				return "", err
			}

			writeMu.Lock()
			defer writeMu.Unlock()

			err = writeTopic(dir, file, payload.Name, payload.Description, payload.Type, payload.Content)
			if err != nil {
				return "", err
			}

			err = upsertIndexLine(dir, file, payload.Name, payload.Description)
			if err != nil {
				return "", err
			}

			result = fmt.Sprintf("saved memory %q", file)

			over, err = indexOverLimit(dir)
			if err != nil {
				return "", err
			}
			if over {
				result += "\n\nwarning: MEMORY.md is over the 200-line/25KB limit; forget a stale entry or shorten descriptions"
			}

			return result, nil
		},
	}
}

func readTool(agentId string) modules.Tool {
	return modules.Tool{
		Name:        "memory_read",
		Description: "Read the full content of a persistent memory note by name.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Name string `json:"name"`
			}
			var dir string
			var file string
			var buf []byte

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}
			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Name == "" {
				return "", fmt.Errorf("name is required")
			}

			dir, err = memoryDir(agentId)
			if err != nil {
				return "", err
			}
			file, err = slugToFile(payload.Name)
			if err != nil {
				return "", err
			}

			buf, err = os.ReadFile(topicPath(dir, file))
			if err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("no memory named %q", payload.Name)
				}

				return "", err
			}

			return string(buf), nil
		},
	}
}

func forgetTool(agentId string) modules.Tool {
	return modules.Tool{
		Name:        "memory_forget",
		Description: "Delete a persistent memory note by name.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Name string `json:"name"`
			}
			var dir string
			var file string

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}
			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Name == "" {
				return "", fmt.Errorf("name is required")
			}

			dir, err = memoryDir(agentId)
			if err != nil {
				return "", err
			}
			file, err = slugToFile(payload.Name)
			if err != nil {
				return "", err
			}

			writeMu.Lock()
			defer writeMu.Unlock()

			err = os.Remove(topicPath(dir, file))
			if err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("no memory named %q", payload.Name)
				}

				return "", err
			}

			err = removeIndexLine(dir, file)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("forgot memory %q", file), nil
		},
	}
}

func LoadIndex(agentId string) string {
	var dir string
	var content string

	var err error

	dir, err = memoryDir(agentId)
	if err != nil {
		return ""
	}

	content, err = readIndex(dir)
	if err != nil || strings.TrimSpace(content) == "" {
		return ""
	}

	return indexHeader + capIndex(content)
}

func Tools(agentId string) []modules.Tool {
	return []modules.Tool{saveTool(agentId), readTool(agentId), forgetTool(agentId)}
}

func List(agentId string) (string, []string, error) {
	var dir string
	var index string
	var entries []os.DirEntry
	var entry os.DirEntry
	var files []string

	var err error

	dir, err = memoryDir(agentId)
	if err != nil {
		return "", nil, err
	}

	index, err = readIndex(dir)
	if err != nil {
		return "", nil, err
	}

	entries, err = os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return index, nil, nil
		}

		return "", nil, err
	}

	for _, entry = range entries {
		if entry.IsDir() || entry.Name() == indexFile || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		files = append(files, entry.Name())
	}

	return index, files, nil
}

func Read(agentId string, name string) (string, error) {
	var dir string
	var file string
	var buf []byte

	var err error

	dir, err = memoryDir(agentId)
	if err != nil {
		return "", err
	}

	file, err = slugToFile(name)
	if err != nil {
		return "", err
	}

	buf, err = os.ReadFile(topicPath(dir, file))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no memory named %q", name)
		}

		return "", err
	}

	return string(buf), nil
}

func Write(agentId string, name string, description string, memType string, content string) error {
	var dir string
	var file string

	var err error

	if description == "" {
		return fmt.Errorf("description is required")
	}
	if !memoryTypes[memType] {
		return fmt.Errorf("type must be one of user, feedback, project, reference")
	}
	if content == "" {
		return fmt.Errorf("content is required")
	}

	dir, err = memoryDir(agentId)
	if err != nil {
		return err
	}

	file, err = slugToFile(name)
	if err != nil {
		return err
	}

	writeMu.Lock()
	defer writeMu.Unlock()

	err = writeTopic(dir, file, name, description, memType, content)
	if err != nil {
		return err
	}

	return upsertIndexLine(dir, file, name, description)
}

func Delete(agentId string, name string) error {
	var dir string
	var file string

	var err error

	dir, err = memoryDir(agentId)
	if err != nil {
		return err
	}

	file, err = slugToFile(name)
	if err != nil {
		return err
	}

	writeMu.Lock()
	defer writeMu.Unlock()

	err = os.Remove(topicPath(dir, file))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no memory named %q", name)
		}

		return err
	}

	return removeIndexLine(dir, file)
}
