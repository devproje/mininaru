// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
)

type yoloRequest struct {
	Mode string `json:"mode" binding:"required"`
	Cwd  string `json:"cwd"`
}

func YoloSet(ctx *gin.Context) {
	var req yoloRequest
	var anchor string

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Mode != core.YoloOff && req.Mode != core.YoloPersist && req.Mode != core.YoloOn {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "mode must be one of off, persist, on"})
		return
	}

	anchor = core.ResolveAnchor(ctx.Request.RemoteAddr, req.Cwd)

	err = core.YoloUpsert(anchor, req.Mode)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"root": anchor, "mode": req.Mode})
}

func YoloGet(ctx *gin.Context) {
	var cwd string
	var anchor string
	var mode string

	cwd = ctx.Query("cwd")
	anchor = core.ResolveAnchor(ctx.Request.RemoteAddr, cwd)
	mode = core.YoloLookup(anchor)

	ctx.JSON(http.StatusOK, gin.H{"root": anchor, "mode": mode})
}
