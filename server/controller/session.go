// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type sessionCreateRequest struct {
	AgentId string `json:"agent_id" binding:"required"`
	Name    string `json:"name"`
	Cwd     string `json:"cwd"`
}

type sessionUpdateRequest struct {
	Name string `json:"name" binding:"required"`
}

func SessionCreate(ctx *gin.Context) {
	var req sessionCreateRequest
	var session core.Session
	var created *core.Session

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session = core.Session{Id: uuid.NewString(), AgentId: req.AgentId, Name: req.Name, Cwd: req.Cwd}

	err = core.SessionCreate(&session)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err = core.SessionRead(session.Id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, created)
}

func SessionRead(ctx *gin.Context) {
	var id string
	var session *core.Session

	var err error

	id = ctx.Param("id")

	session, err = core.SessionRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, session)
}

func SessionList(ctx *gin.Context) {
	var agentId string
	var list []*core.Session

	var err error

	agentId = ctx.Query("agent_id")
	if agentId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	list, err = core.SessionList(agentId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, list)
}

func SessionUpdate(ctx *gin.Context) {
	var id string
	var req sessionUpdateRequest
	var session *core.Session

	var err error

	id = ctx.Param("id")

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = core.SessionUpdate(id, &core.Session{Name: req.Name})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	session, err = core.SessionRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, session)
}

func SessionDelete(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")

	err = core.SessionDelete(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}
