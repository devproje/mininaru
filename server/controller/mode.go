// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
)

type modeRequest struct {
	Mode string `json:"mode" binding:"required"`
	Cwd  string `json:"cwd"`
}

func ModeSet(ctx *gin.Context) {
	var req modeRequest
	var anchor string

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Mode != core.ModeDefault && req.Mode != core.ModePlan && req.Mode != core.ModeAutoPersist && req.Mode != core.ModeFullAuto {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "mode must be one of default, plan, auto_persist, full_auto"})
		return
	}

	anchor = core.ResolveAnchor(ctx.Request.RemoteAddr, req.Cwd)

	err = core.ModeUpsert(anchor, req.Mode)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"root": anchor, "mode": req.Mode})
}

func ModeGet(ctx *gin.Context) {
	var cwd string
	var anchor string
	var mode string

	cwd = ctx.Query("cwd")
	anchor = core.ResolveAnchor(ctx.Request.RemoteAddr, cwd)
	mode = core.ModeLookup(anchor)

	ctx.JSON(http.StatusOK, gin.H{"root": anchor, "mode": mode})
}
