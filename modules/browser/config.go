// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package browser

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/devproje/mininaru/util"
)

type Config struct {
	Channel string `json:"channel,omitempty"`
}

const configPath = "browser.json"

const (
	ChannelChrome   = "chrome"
	ChannelChromium = "chromium"
	ChannelEdge     = "edge"
	ChannelBrave    = "brave"
)

func ValidChannel(channel string) bool {
	switch channel {
	case "", ChannelChrome, ChannelChromium, ChannelEdge, ChannelBrave:
		return true
	default:
		return false
	}
}

func LoadConfig() (Config, error) {
	var buf []byte
	var config Config

	var err error

	buf, err = os.ReadFile(util.Path(configPath))
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}

		return Config{}, err
	}

	err = json.Unmarshal(buf, &config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

func SaveConfig(config Config) error {
	var buf []byte

	var err error

	if !ValidChannel(config.Channel) {
		return fmt.Errorf("unknown browser channel %q", config.Channel)
	}

	buf, err = json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(util.Path(configPath), buf, 0600)
}
