// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"net/http"

	"github.com/devproje/mininaru/modules/browser"
	"github.com/gin-gonic/gin"
)

type browserConfigResponse struct {
	Channel   string `json:"channel"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

func browserResponse(config browser.Config) browserConfigResponse {
	return browserConfigResponse{
		Channel:   config.Channel,
		Path:      browser.ResolvedPath(),
		Available: browser.Available(),
	}
}

func BrowserRead(ctx *gin.Context) {
	var config browser.Config

	var err error

	config, err = browser.LoadConfig()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, browserResponse(config))
}

func BrowserUpdate(ctx *gin.Context) {
	var req struct {
		Channel string `json:"channel"`
	}
	var config browser.Config

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !browser.ValidChannel(req.Channel) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "channel must be one of chrome, chromium, edge, brave, or empty"})
		return
	}

	config, err = browser.LoadConfig()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	config.Channel = req.Channel

	err = browser.SaveConfig(config)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, browserResponse(config))
}
