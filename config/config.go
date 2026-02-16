// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * MiniNaru
 * Copyright (C) 2022-2026 Project_IO
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package config

import (
	_ "embed"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type SSLConfig struct {
	Enable   bool   `toml:"enable"`
	KeyFile  string `toml:"key_file"`
	CertFile string `toml:"cert_file"`
}

type LogConfig struct {
	LogLevel int `toml:"log_level"`
}

type VersionInfo struct {
	Version   string
	Branch    string
	GitHash   string
	BuildTime string
	GoVersion string
}

type ConfigData struct {
	Host    string    `toml:"host"`
	Port    int       `toml:"port"`
	DataDir string    `toml:"datadir"`
	SSL     SSLConfig `toml:"ssl"`
	Log     LogConfig `toml:"log"`

	Ver *VersionInfo `toml:"-"`
}

var Get *ConfigData

//go:embed config.sample.toml
var defaults []byte

func Load(ver *VersionInfo) error {
	var buf, err = os.ReadFile("config.toml")
	if err != nil {
		err = os.WriteFile("config.toml", defaults, 0644)
		if err != nil {
			goto err_io_handle
		}
	}

	Get = &ConfigData{}

	err = toml.Unmarshal(buf, &Get)
	if err != nil {
		goto err_io_handle
	}

	if Get.DataDir == "" {
		Get.DataDir = ".narudata"
	}

	Get.Ver = ver

	_, err = os.Stat(Get.DataDir)
	if err != nil {
		err = os.Mkdir(Get.DataDir, 0700)
		if err != nil {
			goto err_io_handle
		}
	}

	return nil

err_io_handle:
	return err
}
