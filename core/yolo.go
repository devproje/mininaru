// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	YoloOff     = "off"
	YoloPersist = "persist"
	YoloOn      = "on"
)

func YoloLoad() (*DirectoryConfig, error) {
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

func YoloSave(config *DirectoryConfig) error {
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

func YoloUpsert(root, mode string) error {
	var config *DirectoryConfig
	var index int
	var found bool

	var err error

	config, err = YoloLoad()
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

	return YoloSave(config)
}

func coveredBy(root, target string) bool {
	if target == root {
		return true
	}

	return strings.HasPrefix(target, root+string(filepath.Separator))
}

func YoloLookup(target string) string {
	var config *DirectoryConfig
	var entry DirectoryEntry
	var best string
	var mode string

	var err error

	config, err = YoloLoad()
	if err != nil {
		return YoloOff
	}

	mode = YoloOff

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
