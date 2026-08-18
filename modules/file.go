// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/devproje/mininaru/util"
)

const defaultReadChars = 65536
const maxReadChars = 1048576

const maxFileDiffChars = 65536

var fileRevisionMu sync.Mutex

var fileRevisions = make(map[string][sha256.Size]byte)

func fileRevision(buf []byte) [sha256.Size]byte {
	return sha256.Sum256(buf)
}

func rememberFileRevision(path string, buf []byte) {
	fileRevisionMu.Lock()
	fileRevisions[path] = fileRevision(buf)
	fileRevisionMu.Unlock()
}

func requireFileRevision(path string, buf []byte) error {
	var expected [sha256.Size]byte
	var found bool

	expected, found = fileRevisions[path]
	if !found {
		return fmt.Errorf("read %s with file_read before modifying the existing file", path)
	}
	if expected != fileRevision(buf) {
		return fmt.Errorf("%s changed since file_read; read it again before modifying it", path)
	}

	return nil
}

func fileDiff(path, before, after string) string {
	var beforeLines []string
	var afterLines []string
	var prefix int
	var suffix int
	var contextStart int
	var contextEnd int
	var beforeEnd int
	var afterEnd int
	var line string
	var diff strings.Builder
	var result string

	if before != "" {
		beforeLines = strings.Split(strings.TrimSuffix(before, "\n"), "\n")
	}
	if after != "" {
		afterLines = strings.Split(strings.TrimSuffix(after, "\n"), "\n")
	}

	for prefix < len(beforeLines) && prefix < len(afterLines) && beforeLines[prefix] == afterLines[prefix] {
		prefix++
	}
	for suffix < len(beforeLines)-prefix && suffix < len(afterLines)-prefix &&
		beforeLines[len(beforeLines)-1-suffix] == afterLines[len(afterLines)-1-suffix] {
		suffix++
	}

	contextStart = prefix - 3
	if contextStart < 0 {
		contextStart = 0
	}
	contextEnd = suffix
	if contextEnd > 3 {
		contextEnd = 3
	}
	beforeEnd = len(beforeLines) - suffix
	afterEnd = len(afterLines) - suffix

	diff.WriteString("--- a/" + path + "\n")
	diff.WriteString("+++ b/" + path + "\n")
	diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", contextStart+1,
		beforeEnd-contextStart+contextEnd, contextStart+1, afterEnd-contextStart+contextEnd))

	for _, line = range beforeLines[contextStart:prefix] {
		diff.WriteString(" " + line + "\n")
	}
	for _, line = range beforeLines[prefix:beforeEnd] {
		diff.WriteString("-" + line + "\n")
	}
	for _, line = range afterLines[prefix:afterEnd] {
		diff.WriteString("+" + line + "\n")
	}
	for _, line = range beforeLines[beforeEnd : beforeEnd+contextEnd] {
		diff.WriteString(" " + line + "\n")
	}

	result = diff.String()
	if len(result) > maxFileDiffChars {
		return result[:maxFileDiffChars] + "\n[diff truncated]"
	}

	return result
}

func readTextFile(path string) ([]byte, error) {
	var buf []byte

	var err error

	buf, err = os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(buf) {
		return nil, fmt.Errorf("file is not valid UTF-8 text: %s", path)
	}
	if strings.IndexByte(string(buf), 0) >= 0 {
		return nil, fmt.Errorf("binary file is not supported: %s", path)
	}

	return buf, nil
}

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
			"Pass offset and limit to read a range of lines instead of the whole file. " +
			"Always read an existing file before file_edit or file_write; modifications are rejected if it changed afterward.",
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
			buf, err = readTextFile(target)
			if err != nil {
				return "", err
			}
			rememberFileRevision(target, buf)

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
		Name: "file_write",
		Description: "Create or replace a UTF-8 text file relative to the process startup directory. " +
			"Existing files must be read with file_read first and must not have changed since. Returns a unified diff.",
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
			var before []byte
			var after []byte
			var info os.FileInfo
			var mode os.FileMode

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
			if !utf8.ValidString(payload.Content) || strings.IndexByte(payload.Content, 0) >= 0 {
				return "", fmt.Errorf("content must be UTF-8 text")
			}
			target, err = writePath(root, payload.Path)
			if err != nil {
				return "", err
			}

			fileRevisionMu.Lock()
			defer fileRevisionMu.Unlock()

			info, err = os.Stat(target)
			if err == nil {
				before, err = readTextFile(target)
				if err != nil {
					return "", err
				}
				err = requireFileRevision(target, before)
				if err != nil {
					return "", err
				}
				mode = info.Mode().Perm()
			} else if !os.IsNotExist(err) {
				return "", err
			} else {
				mode = 0644
			}

			after = []byte(payload.Content)
			if payload.Append {
				after = append(append([]byte{}, before...), after...)
			}
			err = util.WriteFileAtomic(target, after, mode)
			if err != nil {
				return "", err
			}
			fileRevisions[target] = fileRevision(after)

			return fileDiff(payload.Path, string(before), string(after)), nil
		},
	}
}

func FileEdit(root string) Def {
	return Def{
		Name: "file_edit",
		Description: "Replace an exact string in a text file relative to the process startup directory. " +
			"The file must be read with file_read first and must not have changed since. " +
			"The string must appear exactly once unless replace_all is set. Returns a unified diff.",
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
			var changed string

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
			fileRevisionMu.Lock()
			defer fileRevisionMu.Unlock()

			buf, err = readTextFile(source)
			if err != nil {
				return "", err
			}
			err = requireFileRevision(source, buf)
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

			changed = strings.Replace(string(buf), payload.OldString, payload.NewString, count)
			err = util.WriteFileAtomic(target, []byte(changed), info.Mode().Perm())
			if err != nil {
				return "", err
			}
			fileRevisions[target] = fileRevision([]byte(changed))

			return fileDiff(payload.Path, string(buf), changed), nil
		},
	}
}
