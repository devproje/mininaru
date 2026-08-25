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

type messageCreateRequest struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type messageUpdateRequest struct {
	Content string `json:"content"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}

func MessageCreate(ctx *gin.Context) {
	var sessionId string
	var req messageCreateRequest
	var msg core.Message
	var created *core.Message

	var err error

	sessionId = ctx.Param("id")

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg = core.Message{Id: uuid.NewString(), SessionId: sessionId, Role: req.Role, Content: req.Content}

	err = core.MessageCreate(&msg)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err = core.MessageRead(msg.Id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, created)
}

func MessageRead(ctx *gin.Context) {
	var id string
	var msg *core.Message

	var err error

	id = ctx.Param("id")

	msg, err = core.MessageRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, msg)
}

func MessageList(ctx *gin.Context) {
	var sessionId string
	var list []*core.Message

	var err error

	sessionId = ctx.Param("id")

	list, err = core.MessageList(sessionId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, list)
}

func MessageUpdate(ctx *gin.Context) {
	var id string
	var req messageUpdateRequest
	var msg *core.Message

	var err error

	id = ctx.Param("id")

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = core.MessageUpdate(id, &core.Message{Content: req.Content, Status: req.Status, Error: req.Error})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	msg, err = core.MessageRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, msg)
}

func MessageDelete(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")

	err = core.MessageDelete(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}
