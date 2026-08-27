// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devproje/mininaru/modules"
	"github.com/devproje/mininaru/util"
)

const ToolName = "skill"

const maxBundleEntries = 32

const skillFooter = `Companion files in this bundle are listed above. Read one with the skill tool's
path argument, and run a script with bash_exec using an absolute path under the
bundle directory.`

func bundleListing(dir string) string {
	var entries []os.DirEntry
	var entry os.DirEntry
	var names []string

	var err error

	entries, err = os.ReadDir(dir)
	if err != nil {
		return ""
	}

	sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })

	for _, entry = range entries {
		if entry.Name() == skillFile || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if len(names) >= maxBundleEntries {
			names = append(names, "...")
			break
		}

		if entry.IsDir() {
			names = append(names, entry.Name()+"/")
			continue
		}

		names = append(names, entry.Name())
	}

	return strings.Join(names, ", ")
}

func bundlePath(entry *Skill, rel string) (string, error) {
	var segments []string
	var segment string

	var err error

	segments = strings.Split(filepath.ToSlash(rel), "/")

	for _, segment = range segments {
		err = util.SafeSegment(segment)
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(segment, ".") {
			return "", fmt.Errorf("hidden path is not readable: %q", rel)
		}
	}

	return filepath.Join(append([]string{entry.Path}, segments...)...), nil
}

func skillCompanion(entry *Skill, rel string) (string, error) {
	var target string
	var buf []byte

	var err error

	target, err = bundlePath(entry, rel)
	if err != nil {
		return "", err
	}

	buf, err = os.ReadFile(target)
	if err != nil {
		return "", err
	}

	if len(buf) > maxSkillBody {
		return fmt.Sprintf("skill: %s\npath: %s\n\n%s\n[truncated]", entry.Name, target, string(buf[:maxSkillBody])), nil
	}

	return fmt.Sprintf("skill: %s\npath: %s\n\n%s", entry.Name, target, string(buf)), nil
}

func skillBody(entry *Skill) string {
	var builder strings.Builder
	var listing string

	builder.WriteString("skill: " + entry.Name + "\n")
	builder.WriteString("path: " + entry.Path + "\n")

	listing = bundleListing(entry.Path)
	if listing != "" {
		builder.WriteString("files: " + listing + "\n")
	}

	builder.WriteString("\n" + entry.Body)

	if listing != "" {
		builder.WriteString("\n\n" + skillFooter)
	}

	return builder.String()
}

func Result(name, rel string) (string, error) {
	var entry *Skill

	entry = Find(name)
	if entry == nil {
		return "", fmt.Errorf("unknown skill %q, available: %s", name, strings.Join(Names(), ", "))
	}

	if rel != "" {
		return skillCompanion(entry, rel)
	}

	return skillBody(entry), nil
}

func Tool() modules.Tool {
	return modules.Tool{
		Name:        ToolName,
		Description: "Load the full instructions for a named skill listed in the system prompt.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Name of a skill listed in the system prompt."},
				"path": map[string]any{"type": "string", "description": "Optional companion file inside the skill bundle, relative to the bundle directory."},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
		Permission: modules.PermissionSafe,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Name string `json:"name"`
				Path string `json:"path"`
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

			if strings.TrimSpace(payload.Name) == "" {
				return "", fmt.Errorf("name is required")
			}

			return Result(strings.TrimSpace(payload.Name), strings.TrimSpace(payload.Path))
		},
	}
}
