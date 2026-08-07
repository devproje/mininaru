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

func FileRead(root string) Def {
	return Def{
		Name:        "file_read",
		Description: "Read a UTF-8 text file relative to the process startup directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":      map[string]any{"type": "string"},
				"max_chars": map[string]any{"type": "integer", "minimum": 1, "maximum": maxReadChars},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
		Permission: PermissionDangerous,
		Execute: func(ctx context.Context, arguments string) (string, error) {
			var payload struct {
				Path     string `json:"path"`
				MaxChars int    `json:"max_chars"`
			}
			var target string
			var buf []byte

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
			if len(buf) > payload.MaxChars {
				return string(buf[:payload.MaxChars]) + "\n[truncated]", nil
			}

			return string(buf), nil
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
