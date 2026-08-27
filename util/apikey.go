// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
)

const apiKeyFile = "mininaru.key"
const apiKeyBytes = 32

func generateAPIKey() (string, error) {
	var raw []byte

	var err error

	raw = make([]byte, apiKeyBytes)

	_, err = rand.Read(raw)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}

func APIKey() (string, error) {
	var path string
	var data []byte
	var key string

	var err error

	path = Path(apiKeyFile)

	data, err = os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	if !os.IsNotExist(err) {
		return "", err
	}

	key, err = generateAPIKey()
	if err != nil {
		return "", err
	}

	err = WriteFileAtomic(path, []byte(key+"\n"), 0600)
	if err != nil {
		return "", err
	}

	return key, nil
}
