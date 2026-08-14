// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const defaultGlobResults = 200

const defaultGrepResults = 100

const maxSearchResults = 1000

const maxSearchFileBytes = 5 << 20

const binarySniffBytes = 8192

const maxGrepLineChars = 512

var searchSkipDirs map[string]bool = map[string]bool{"node_modules": true, "vendor": true}

func searchRoot(root, rel string) (string, error) {
	var base string

	var err error

	base, err = toolRoot(root)
	if err != nil {
		return "", err
	}

	if rel == "" {
		return base, nil
	}

	return readPath(base, rel)
}

func searchSkip(name string) bool {
	if searchSkipDirs[name] {
		return true
	}

	return strings.HasPrefix(name, ".")
}

func searchWalk(ctx context.Context, base string, visit func(path, rel string) (bool, error)) error {
	return filepath.WalkDir(base, func(current string, entry fs.DirEntry, walkErr error) error {
		var rel string
		var keep bool

		var err error

		if walkErr != nil {
			return nil
		}

		if err = ctx.Err(); err != nil {
			return err
		}

		if current == base {
			return nil
		}

		if searchSkip(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if !entry.Type().IsRegular() {
			return nil
		}

		rel, err = filepath.Rel(base, current)
		if err != nil {
			return nil
		}

		keep, err = visit(current, filepath.ToSlash(rel))
		if err != nil {
			return err
		}

		if !keep {
			return filepath.SkipAll
		}

		return nil
	})
}

func segmentMatch(pattern, name []string) bool {
	var index int
	var matched bool

	var err error

	if len(pattern) == 0 {
		return len(name) == 0
	}

	if pattern[0] == "**" {
		for index = 0; index <= len(name); index++ {
			if segmentMatch(pattern[1:], name[index:]) {
				return true
			}
		}

		return false
	}

	if len(name) == 0 {
		return false
	}

	matched, err = filepath.Match(pattern[0], name[0])
	if err != nil || !matched {
		return false
	}

	return segmentMatch(pattern[1:], name[1:])
}

func pathMatch(pattern, name string) bool {
	return segmentMatch(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func Glob(root string) Def {
	return Def{
		Name: "glob",
		Description: "List files by path pattern under the process startup directory. " +
			"Use ** to match across directories, for example **/*.go.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string"},
				"path":        map[string]any{"type": "string"},
				"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": maxSearchResults},
			},
			"required":             []string{"pattern"},
			"additionalProperties": false,
		},
		Permission: PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Pattern    string `json:"pattern"`
				Path       string `json:"path"`
				MaxResults int    `json:"max_results"`
			}
			var base string
			var found []string

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Pattern == "" {
				return "", fmt.Errorf("pattern is required")
			}
			if payload.MaxResults <= 0 {
				payload.MaxResults = defaultGlobResults
			}
			if payload.MaxResults > maxSearchResults {
				return "", fmt.Errorf("max_results cannot exceed %d", maxSearchResults)
			}

			base, err = searchRoot(root, payload.Path)
			if err != nil {
				return "", err
			}

			err = searchWalk(ctx, base, func(current, rel string) (bool, error) {
				if !pathMatch(payload.Pattern, rel) {
					return true, nil
				}

				found = append(found, rel)

				return len(found) < payload.MaxResults, nil
			})
			if err != nil {
				return "", err
			}

			if len(found) == 0 {
				return "no file matched " + payload.Pattern, nil
			}

			sort.Strings(found)

			if len(found) >= payload.MaxResults {
				return strings.Join(found, "\n") + "\n[truncated]", nil
			}

			return strings.Join(found, "\n"), nil
		},
	}
}

func grepExcerpt(line string) string {
	var runes []rune

	line = strings.TrimSpace(line)

	runes = []rune(line)
	if len(runes) <= maxGrepLineChars {
		return line
	}

	return string(runes[:maxGrepLineChars]) + "…"
}

func grepFile(path, rel string, expr *regexp.Regexp, budget int) []string {
	var info os.FileInfo
	var buf []byte
	var lines []string
	var index int
	var line string
	var found []string

	var err error

	info, err = os.Stat(path)
	if err != nil || info.Size() > maxSearchFileBytes {
		return nil
	}

	buf, err = os.ReadFile(path)
	if err != nil {
		return nil
	}

	if bytes.IndexByte(buf[:min(len(buf), binarySniffBytes)], 0) >= 0 {
		return nil
	}

	lines = strings.Split(string(buf), "\n")

	for index, line = range lines {
		line = strings.TrimRight(line, "\r")
		if !expr.MatchString(line) {
			continue
		}

		found = append(found, fmt.Sprintf("%s:%d:%s", rel, index+1, grepExcerpt(line)))
		if len(found) >= budget {
			return found
		}
	}

	return found
}

func Grep(root string) Def {
	return Def{
		Name: "grep",
		Description: "Search file contents by regular expression under the process startup directory. " +
			"Returns matching lines as path:line:text.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string"},
				"path":        map[string]any{"type": "string"},
				"glob":        map[string]any{"type": "string"},
				"ignore_case": map[string]any{"type": "boolean"},
				"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": maxSearchResults},
			},
			"required":             []string{"pattern"},
			"additionalProperties": false,
		},
		Permission: PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Pattern    string `json:"pattern"`
				Path       string `json:"path"`
				Glob       string `json:"glob"`
				IgnoreCase bool   `json:"ignore_case"`
				MaxResults int    `json:"max_results"`
			}
			var source string
			var expr *regexp.Regexp
			var base string
			var found []string

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}

			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Pattern == "" {
				return "", fmt.Errorf("pattern is required")
			}
			if payload.MaxResults <= 0 {
				payload.MaxResults = defaultGrepResults
			}
			if payload.MaxResults > maxSearchResults {
				return "", fmt.Errorf("max_results cannot exceed %d", maxSearchResults)
			}

			source = payload.Pattern
			if payload.IgnoreCase {
				source = "(?i)" + source
			}

			expr, err = regexp.Compile(source)
			if err != nil {
				return "", fmt.Errorf("invalid pattern: %w", err)
			}

			base, err = searchRoot(root, payload.Path)
			if err != nil {
				return "", err
			}

			err = searchWalk(ctx, base, func(current, rel string) (bool, error) {
				if payload.Glob != "" && !pathMatch(payload.Glob, rel) {
					return true, nil
				}

				found = append(found, grepFile(current, rel, expr, payload.MaxResults-len(found))...)

				return len(found) < payload.MaxResults, nil
			})
			if err != nil {
				return "", err
			}

			if len(found) == 0 {
				return "no match for " + payload.Pattern, nil
			}

			if len(found) >= payload.MaxResults {
				return strings.Join(found, "\n") + "\n[truncated]", nil
			}

			return strings.Join(found, "\n"), nil
		},
	}
}
