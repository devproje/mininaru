// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const updateCacheFile = "update.json"

const UpdateCacheTTL = 24 * time.Hour

type UpdateCache struct {
	Tag       string `json:"tag"`
	CheckedAt int64  `json:"checked_at"`
}

func updateCachePath() string {
	return Path(updateCacheFile)
}

func UpdateCacheRead() UpdateCache {
	var current UpdateCache
	var buf []byte

	var err error

	buf, err = os.ReadFile(updateCachePath())
	if err != nil {
		return current
	}

	err = json.Unmarshal(buf, &current)
	if err != nil {
		return UpdateCache{}
	}

	return current
}

func UpdateCacheWrite(tag string) {
	var buf []byte

	var err error

	buf, err = json.MarshalIndent(UpdateCache{Tag: tag, CheckedAt: time.Now().Unix()}, "", "    ")
	if err != nil {
		return
	}

	err = WriteFileAtomic(updateCachePath(), buf, 0600)
	if err != nil {
		Log.Debug("caching the update check failed", "error", err)
	}
}

func UpdateNotice() string {
	var current UpdateCache

	if AppVersion == "dev" || os.Getenv("MININARU_NO_UPDATE_CHECK") != "" {
		return ""
	}

	current = UpdateCacheRead()
	if current.Tag == "" || current.Tag == AppVersion {
		return ""
	}

	return fmt.Sprintf("a newer version is available: %s (run `mininaru update`)", current.Tag)
}
