// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devproje/mininaru/modules/client"
)

type tuiClipboardResolvedMsg struct {
	path    string
	cleanup bool
	err     error
}

var clipboardImageTools = [][]string{
	{"wl-paste", "--type", "image/png"},
	{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"},
	{"pngpaste", "-"},
}

var clipboardFileListTools = [][]string{
	{"wl-paste", "--type", "text/uri-list"},
	{"xclip", "-selection", "clipboard", "-t", "text/uri-list", "-o"},
}

var pastableImageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}

func hasImageExt(path string) bool {
	var lower string
	var ext string

	lower = strings.ToLower(path)

	for _, ext = range pastableImageExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	return false
}

func runClipboardTool(tools [][]string) ([]byte, error) {
	var tool []string
	var out []byte

	var err error

	for _, tool = range tools {
		_, err = exec.LookPath(tool[0])
		if err != nil {
			continue
		}

		out, err = exec.Command(tool[0], tool[1:]...).Output()
		if err == nil && len(out) > 0 {
			return out, nil
		}
	}

	return nil, fmt.Errorf("not available")
}

func clipboardFilePath() (string, error) {
	var raw []byte
	var line string
	var path string
	var parsed *url.URL

	var err error

	raw, err = runClipboardTool(clipboardFileListTools)
	if err != nil {
		return "", err
	}

	for _, line = range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		path = line
		if strings.HasPrefix(line, "file://") {
			parsed, err = url.Parse(line)
			if err != nil {
				continue
			}

			path = parsed.Path
		}

		if hasImageExt(path) {
			_, err = os.Stat(path)
			if err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("no image file on the clipboard")
}

func clipboardImageFile() (string, bool, error) {
	var path string
	var data []byte
	var f *os.File

	var err error

	path, err = clipboardFilePath()
	if err == nil {
		return path, false, nil
	}

	data, err = runClipboardTool(clipboardImageTools)
	if err != nil {
		return "", false, fmt.Errorf("no image on the clipboard")
	}

	f, err = os.CreateTemp("", "mininaru-paste-*.png")
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		os.Remove(f.Name())

		return "", false, err
	}

	return f.Name(), true, nil
}

func resolveClipboardImageCmd() tea.Cmd {
	return func() tea.Msg {
		var path string
		var cleanup bool
		var err error

		path, cleanup, err = clipboardImageFile()

		return tuiClipboardResolvedMsg{path: path, cleanup: cleanup, err: err}
	}
}

func uploadImageCmd(base, apiKey, session, path string, cleanup bool) tea.Cmd {
	return func() tea.Msg {
		var id string
		var err error

		if cleanup {
			defer os.Remove(path)
		}

		id, err = client.Upload(base, apiKey, session, path)

		return tuiImgUploadedMsg{id: id, err: err}
	}
}

func extractPastedImagePath(s string) (string, bool) {
	var trimmed string
	var parsed *url.URL

	var err error

	trimmed = strings.TrimSpace(s)
	if trimmed == "" || strings.ContainsAny(trimmed, "\n\r") {
		return "", false
	}

	if len(trimmed) >= 2 {
		if (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') || (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') {
			trimmed = trimmed[1 : len(trimmed)-1]
		}
	}

	if strings.HasPrefix(trimmed, "file://") {
		parsed, err = url.Parse(trimmed)
		if err != nil {
			return "", false
		}

		trimmed = parsed.Path
	}

	if !hasImageExt(trimmed) {
		return "", false
	}

	_, err = os.Stat(trimmed)
	if err != nil {
		return "", false
	}

	return trimmed, true
}
