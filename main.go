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

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"git.wh64.net/naru-studio/mininaru/config"
	"git.wh64.net/naru-studio/mininaru/core"
	"git.wh64.net/naru-studio/mininaru/modules/agent"
	"git.wh64.net/naru-studio/mininaru/modules/database"
	"git.wh64.net/naru-studio/mininaru/modules/webserver"
	"github.com/gin-gonic/gin"
)

var (
	version   string
	branch    string
	hash      string
	buildTime string
	goVersion string
)

func main() {
	var quit chan os.Signal
	var err error

	err = config.Load(&config.VersionInfo{
		Version:   version,
		Branch:    branch,
		GitHash:   hash,
		BuildTime: buildTime,
		GoVersion: goVersion,
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	core.NaruCore = core.NewMiniNaru()

	gin.SetMode(gin.ReleaseMode)
	for _, arg := range os.Args {
		if arg == "--debug" || arg == "-d" {
			gin.SetMode(gin.DebugMode)
		}
	}

	core.NaruCore.Insmod(database.Database)

	core.NaruCore.Insmod(agent.Agent)
	core.NaruCore.Insmod(webserver.WebServer)

	err = core.NaruCore.Init()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	quit = make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	_ = core.NaruCore.Destroy()
}
