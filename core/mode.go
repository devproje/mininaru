// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devproje/mininaru/util"
)

type DirectoryEntry struct {
	Root      string `json:"root"`
	Mode      string `json:"mode"`
	UpdatedAt string `json:"updated_at"`
}

type DirectoryConfig struct {
	Entries []DirectoryEntry `json:"entries"`
}

const directoryPath = "directory.json"

const (
	ModeDefault     = "default"
	ModePlan        = "plan"
	ModeAutoPersist = "auto_persist"
	ModeFullAuto    = "full_auto"
)

var sessionModeOverrides sync.Map

func SetSessionModeOverride(sessionId, mode string) {
	sessionModeOverrides.Store(sessionId, mode)
}

func SessionModeOverride(sessionId string) (string, bool) {
	var stored any
	var mode string
	var ok bool

	stored, ok = sessionModeOverrides.Load(sessionId)
	if !ok {
		return "", false
	}

	mode, ok = stored.(string)

	return mode, ok
}

func ClearSessionModeOverride(sessionId string) {
	sessionModeOverrides.Delete(sessionId)
}

func ModeLoad() (*DirectoryConfig, error) {
	var path string
	var buf []byte
	var config DirectoryConfig

	var err error

	path = util.Path(directoryPath)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		return &DirectoryConfig{}, nil
	}

	err = json.Unmarshal(buf, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func ModeSave(config *DirectoryConfig) error {
	var path string
	var buf []byte

	var err error

	path = util.Path(directoryPath)
	buf, err = json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(path, buf, 0600)
}

func ModeUpsert(root, mode string) error {
	var config *DirectoryConfig
	var index int
	var found bool

	var err error

	config, err = ModeLoad()
	if err != nil {
		return err
	}

	for index = range config.Entries {
		if config.Entries[index].Root != root {
			continue
		}

		config.Entries[index].Mode = mode
		config.Entries[index].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		found = true

		break
	}

	if !found {
		config.Entries = append(config.Entries, DirectoryEntry{
			Root: root, Mode: mode, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	return ModeSave(config)
}

func coveredBy(root, target string) bool {
	if target == root {
		return true
	}

	return strings.HasPrefix(target, root+string(filepath.Separator))
}

func ModeLookup(target string) string {
	var config *DirectoryConfig
	var mode string
	var entry DirectoryEntry
	var best string

	var err error

	config, err = ModeLoad()
	if err != nil {
		return ModeDefault
	}

	mode = ModeDefault

	for _, entry = range config.Entries {
		if !coveredBy(entry.Root, target) {
			continue
		}
		if len(entry.Root) < len(best) {
			continue
		}

		best = entry.Root
		mode = entry.Mode
	}

	return mode
}

func modeEscapesPath(anchor, path string) bool {
	var full string

	if filepath.IsAbs(path) {
		return true
	}

	full = filepath.Join(anchor, path)

	return full != anchor && !coveredBy(anchor, full)
}

func IsIOTool(name string) bool {
	switch name {
	case "file_read", "file_write", "file_edit":
		return true
	default:
		return false
	}
}

func IsReadOnlyTool(name string) bool {
	switch name {
	case "file_read", "browser_read", "browser_screenshot":
		return true
	default:
		return false
	}
}

func ModeEscapesAnchor(anchor, arguments string) bool {
	var payload struct {
		Path string `json:"path"`
	}

	var err error

	err = json.Unmarshal([]byte(arguments), &payload)
	if err != nil {
		return false
	}

	return modeEscapesPath(anchor, payload.Path)
}

func IsLoopbackAddr(remoteAddr string) bool {
	var host string
	var ip net.IP

	var err error

	host, _, err = net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	if host == "localhost" {
		return true
	}

	ip = net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback()
}

func ResolveAnchor(remoteAddr, clientCwd string) string {
	var home string

	var err error

	if IsLoopbackAddr(remoteAddr) && clientCwd != "" {
		return clientCwd
	}

	home, err = os.UserHomeDir()
	if err != nil {
		return "."
	}

	return home
}

func allowedAnchor(remoteAddr, clientCwd string) string {
	var home string

	var err error

	if clientCwd == "" {
		return ""
	}

	if !filepath.IsAbs(clientCwd) {
		return ""
	}

	clientCwd = filepath.Clean(clientCwd)

	if IsLoopbackAddr(remoteAddr) {
		return clientCwd
	}

	home, err = os.UserHomeDir()
	if err != nil {
		return ""
	}

	if !coveredBy(filepath.Clean(home), clientCwd) {
		return ""
	}

	return clientCwd
}

func SessionAnchor(session *Session, remoteAddr, clientCwd string) string {
	var anchor string

	var err error

	if session.Cwd != "" {
		return session.Cwd
	}

	anchor = allowedAnchor(remoteAddr, clientCwd)
	if anchor == "" {
		return ""
	}

	err = SessionUpdate(session.Id, &Session{Cwd: anchor})
	if err != nil {
		util.Log.Error("session cwd pin failed", "session", session.Id, "error", err)
	}

	session.Cwd = anchor

	return anchor
}
