// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package shell

import (
	"encoding/json"
	"os"

	"github.com/devproje/mininaru/util"
)

const preferencesPath = "shell.json"

type preferences struct {
	Agent string `json:"agent,omitempty"`
}

func loadPreferences() (*preferences, error) {
	var path string
	var buf []byte
	var prefs preferences

	var err error

	path = util.Path(preferencesPath)
	buf, err = os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &preferences{}, nil
		}

		return nil, err
	}

	err = json.Unmarshal(buf, &prefs)
	if err != nil {
		return nil, err
	}

	return &prefs, nil
}

func savePreferences(prefs *preferences) error {
	var path string
	var buf []byte

	var err error

	path = util.Path(preferencesPath)
	buf, err = json.MarshalIndent(prefs, "", "    ")
	if err != nil {
		return err
	}

	return util.WriteFileAtomic(path, buf, 0600)
}
