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

type agentCreateRequest struct {
	Name          string `json:"name" binding:"required"`
	Model         string `json:"model" binding:"required"`
	Soul          string `json:"soul"`
	ThinkingLevel string `json:"thinking_level"`
	MaxContext    uint64 `json:"max_context"`
}

type agentUpdateRequest struct {
	Name          string `json:"name"`
	Model         string `json:"model"`
	Soul          string `json:"soul"`
	ThinkingLevel string `json:"thinking_level"`
	MaxContext    uint64 `json:"max_context"`
}

func AgentCreate(ctx *gin.Context) {
	var req agentCreateRequest
	var agent core.Agent
	var created *core.Agent

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent = core.Agent{
		Id:            uuid.NewString(),
		Name:          req.Name,
		Model:         req.Model,
		Soul:          req.Soul,
		ThinkingLevel: req.ThinkingLevel,
		MaxContext:    req.MaxContext,
	}

	err = core.AgentCreate(&agent)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err = core.AgentRead(agent.Id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, created)
}

func AgentRead(ctx *gin.Context) {
	var id string
	var agent *core.Agent

	var err error

	id = ctx.Param("id")

	agent, err = core.AgentRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, agent)
}

func AgentList(ctx *gin.Context) {
	var list []*core.Agent

	var err error

	list, err = core.AgentList()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, list)
}

func AgentUpdate(ctx *gin.Context) {
	var id string
	var req agentUpdateRequest
	var agent *core.Agent

	var err error

	id = ctx.Param("id")

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = core.AgentUpdate(id, &core.Agent{
		Name:          req.Name,
		Model:         req.Model,
		Soul:          req.Soul,
		ThinkingLevel: req.ThinkingLevel,
		MaxContext:    req.MaxContext,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	agent, err = core.AgentRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, agent)
}

func AgentDelete(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")

	err = core.AgentDelete(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}
