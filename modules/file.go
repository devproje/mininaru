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
)

const defaultReadChars = 65536
const maxReadChars = 1048576

func toolRoot(root string) (string, error) {
	var resolved string

	var err error

	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	resolved, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}

	return filepath.Abs(resolved)
}

func readPath(root, path string) (string, error) {
	var base string
	var target string
	var resolved string

	var err error

	base, err = toolRoot(root)
	if err != nil {
		return "", err
	}
	target, err = util.SafeJoin(base, path)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	if resolved != base && !strings.HasPrefix(resolved, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes working directory: %q", path)
	}

	return resolved, nil
}

func writePath(root, path string) (string, error) {
	var base string
	var target string
	var parent string
	var resolvedParent string
	var info os.FileInfo
	var resolvedTarget string

	var err error

	base, err = toolRoot(root)
	if err != nil {
		return "", err
	}
	target, err = util.SafeJoin(base, path)
	if err != nil {
		return "", err
	}
	parent = filepath.Dir(target)
	resolvedParent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if resolvedParent != base && !strings.HasPrefix(resolvedParent, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes working directory: %q", path)
	}
	info, err = os.Lstat(target)
	if err == nil {
		resolvedTarget, err = filepath.EvalSymlinks(target)
		if err != nil {
			return "", err
		}
		if resolvedTarget != base && !strings.HasPrefix(resolvedTarget, base+string(filepath.Separator)) {
			return "", fmt.Errorf("path escapes working directory: %q", path)
		}
		if info.IsDir() {
			return "", fmt.Errorf("path is a directory: %q", path)
		}
		return resolvedTarget, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	return filepath.Join(resolvedParent, filepath.Base(target)), nil
}

func sliceLines(text string, offset, limit int) string {
	var lines []string

	if offset <= 0 && limit <= 0 {
		return text
	}

	lines = strings.Split(text, "\n")

	if offset > 0 {
		if offset > len(lines) {
			return ""
		}

		lines = lines[offset-1:]
	}

	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}

	return strings.Join(lines, "\n")
}

func FileRead(root string) Def {
	return Def{
		Name: "file_read",
		Description: "Read a UTF-8 text file relative to the process startup directory. " +
			"Pass offset and limit to read a range of lines instead of the whole file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":      map[string]any{"type": "string"},
				"offset":    map[string]any{"type": "integer", "minimum": 1},
				"limit":     map[string]any{"type": "integer", "minimum": 1},
				"max_chars": map[string]any{"type": "integer", "minimum": 1, "maximum": maxReadChars},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
		Permission: PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Path     string `json:"path"`
				Offset   int    `json:"offset"`
				Limit    int    `json:"limit"`
				MaxChars int    `json:"max_chars"`
			}
			var target string
			var buf []byte
			var text string

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}
			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Path == "" {
				return "", fmt.Errorf("path is required")
			}
			if payload.MaxChars <= 0 {
				payload.MaxChars = defaultReadChars
			}
			if payload.MaxChars > maxReadChars {
				return "", fmt.Errorf("max_chars cannot exceed %d", maxReadChars)
			}

			target, err = readPath(root, payload.Path)
			if err != nil {
				return "", err
			}
			buf, err = os.ReadFile(target)
			if err != nil {
				return "", err
			}

			text = sliceLines(string(buf), payload.Offset, payload.Limit)
			if len(text) > payload.MaxChars {
				return text[:payload.MaxChars] + "\n[truncated]", nil
			}

			return text, nil
		},
	}
}

func FileWrite(root string) Def {
	return Def{
		Name:        "file_write",
		Description: "Write a text file relative to the process startup directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
				"append":  map[string]any{"type": "boolean"},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": false,
		},
		Permission: PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Path    string `json:"path"`
				Content string `json:"content"`
				Append  bool   `json:"append"`
			}
			var target string
			var flags int
			var file *os.File

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}
			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Path == "" {
				return "", fmt.Errorf("path is required")
			}
			target, err = writePath(root, payload.Path)
			if err != nil {
				return "", err
			}

			flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
			if payload.Append {
				flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
			}
			file, err = os.OpenFile(target, flags, 0644)
			if err != nil {
				return "", err
			}
			defer file.Close()

			_, err = file.WriteString(payload.Content)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("wrote %d bytes to %s", len(payload.Content), payload.Path), nil
		},
	}
}

func FileEdit(root string) Def {
	return Def{
		Name: "file_edit",
		Description: "Replace an exact string in a text file relative to the process startup directory. " +
			"The string must appear exactly once unless replace_all is set.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":        map[string]any{"type": "string"},
				"old_string":  map[string]any{"type": "string"},
				"new_string":  map[string]any{"type": "string"},
				"replace_all": map[string]any{"type": "boolean"},
			},
			"required":             []string{"path", "old_string", "new_string"},
			"additionalProperties": false,
		},
		Permission: PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Path       string `json:"path"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			var source string
			var buf []byte
			var count int
			var target string
			var info os.FileInfo

			var err error

			if err = ctx.Err(); err != nil {
				return "", err
			}
			err = json.Unmarshal([]byte(arguments), &payload)
			if err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if payload.Path == "" {
				return "", fmt.Errorf("path is required")
			}
			if payload.OldString == "" {
				return "", fmt.Errorf("old_string is required")
			}
			if payload.OldString == payload.NewString {
				return "", fmt.Errorf("old_string and new_string are identical")
			}

			source, err = readPath(root, payload.Path)
			if err != nil {
				return "", err
			}
			buf, err = os.ReadFile(source)
			if err != nil {
				return "", err
			}

			count = strings.Count(string(buf), payload.OldString)
			if count == 0 {
				return "", fmt.Errorf("old_string not found in %s", payload.Path)
			}
			if count > 1 && !payload.ReplaceAll {
				return "", fmt.Errorf("old_string matches %d times in %s, pass more surrounding context or set replace_all",
					count, payload.Path)
			}

			target, err = writePath(root, payload.Path)
			if err != nil {
				return "", err
			}
			info, err = os.Stat(target)
			if err != nil {
				return "", err
			}

			err = os.WriteFile(target, []byte(strings.Replace(string(buf), payload.OldString, payload.NewString, count)),
				info.Mode().Perm())
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("replaced %d occurrence(s) in %s", count, payload.Path), nil
		},
	}
}
