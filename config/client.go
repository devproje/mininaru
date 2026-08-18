// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/devproje/mininaru/util"
)

type Thinking struct {
	Level string `json:"level"`
	Show  bool   `json:"show"`
}

type Context struct {
	Compact bool `json:"compact"`
}

type Tools struct {
	Enabled bool `json:"enabled"`
}

type Update struct {
	Check bool `json:"check"`
}

type Server struct {
	Address string `json:"address"`
}

type ClientConfig struct {
	Thinking Thinking `json:"thinking"`
	Context  Context  `json:"context"`
	Tools    Tools    `json:"tools"`
	Update   Update   `json:"update"`
	Server   Server   `json:"server"`
}

const CLIENT_PATH = "client.json"

const NoUpdateCheckEnv = "MININARU_NO_UPDATE_CHECK"

const (
	ThinkingOff    = "off"
	ThinkingLow    = "low"
	ThinkingMedium = "medium"
	ThinkingHigh   = "high"
	ThinkingMax    = "max"
)

var Client ClientConfig

var AllowDangerousTools bool

var defaultClient ClientConfig = ClientConfig{
	Thinking: Thinking{Level: ThinkingOff, Show: true},
	Context:  Context{Compact: true},
	Tools:    Tools{Enabled: true},
	Update:   Update{Check: true},
}

func ThinkingLevels() []string {
	return []string{ThinkingOff, ThinkingLow, ThinkingMedium, ThinkingHigh, ThinkingMax}
}

func ThinkingValid(level string) bool {
	var cur string

	for _, cur = range ThinkingLevels() {
		if cur != level {
			continue
		}

		return true
	}

	return false
}

func ThinkingEnabled() bool {
	return Client.Thinking.Level != "" && Client.Thinking.Level != ThinkingOff
}

func UpdateCheckEnabled() bool {
	if os.Getenv(NoUpdateCheckEnv) != "" {
		return false
	}

	return Client.Update.Check
}

func ClientInit() error {
	var path string
	var buf []byte

	var err error

	Client = defaultClient

	path = util.Path(CLIENT_PATH)
	buf, err = os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		buf, _ = json.MarshalIndent(defaultClient, "", "    ")

		err = util.WriteFileAtomic(path, buf, 0600)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal(buf, &Client)
	if err != nil {
		return err
	}

	Client.Thinking.Level = strings.ToLower(Client.Thinking.Level)

	if !ThinkingValid(Client.Thinking.Level) {
		util.Log.Warn("ignoring an invalid thinking level",
			"config", CLIENT_PATH, "thinking_level", Client.Thinking.Level, "fallback", defaultClient.Thinking.Level)

		Client.Thinking.Level = defaultClient.Thinking.Level
	}

	return nil
}

func ClientSave() error {
	var path string
	var buf []byte

	var err error

	path = util.Path(CLIENT_PATH)
	buf, err = json.MarshalIndent(Client, "", "    ")
	if err != nil {
		return err
	}

	err = util.WriteFileAtomic(path, buf, 0600)
	if err != nil {
		return err
	}

	return nil
}
