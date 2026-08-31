// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/memory"
	"github.com/gin-gonic/gin"
)

type memoryWriteRequest struct {
	Description string `json:"description" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Content     string `json:"content" binding:"required"`
}

func agentExists(ctx *gin.Context, id string) bool {
	var err error

	_, err = core.AgentRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return false
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}

	return true
}

func MemoryList(ctx *gin.Context) {
	var id string
	var index string
	var files []string

	var err error

	id = ctx.Param("id")
	if !agentExists(ctx, id) {
		return
	}

	index, files, err = memory.List(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"index": index, "files": files})
}

func MemoryRead(ctx *gin.Context) {
	var id string
	var content string

	var err error

	id = ctx.Param("id")
	if !agentExists(ctx, id) {
		return
	}

	content, err = memory.Read(id, ctx.Param("file"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"content": content})
}

func MemoryWrite(ctx *gin.Context) {
	var id string
	var req memoryWriteRequest

	var err error

	id = ctx.Param("id")
	if !agentExists(ctx, id) {
		return
	}

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = memory.Write(id, ctx.Param("file"), req.Description, req.Type, req.Content)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}

func MemoryDelete(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")
	if !agentExists(ctx, id) {
		return
	}

	err = memory.Delete(id, ctx.Param("file"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}
