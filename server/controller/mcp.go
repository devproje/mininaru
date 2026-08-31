// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"context"
	"net/http"

	"github.com/devproje/mininaru/modules/mcp"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

type mcpEntry struct {
	Server mcp.Server `json:"server"`
	Status mcp.Status `json:"status"`
}

func mcpStatusFor(all []mcp.Status, name string) mcp.Status {
	var item mcp.Status

	for _, item = range all {
		if item.Name == name {
			return item
		}
	}

	return mcp.Status{Name: name}
}

func mcpIndexOf(name string) int {
	var i int

	for i = range mcp.Loaded.Servers {
		if mcp.Loaded.Servers[i].Name == name {
			return i
		}
	}

	return -1
}

func mcpReconnect() {
	var err error

	err = mcp.Reload(context.Background())
	if err != nil {
		util.Log.Warn("mcp reload after api change failed", "error", err)
	}
}

func mcpSetEnabled(ctx *gin.Context, name string, enabled bool) {
	var index int

	var err error

	err = mcp.Load()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	index = mcpIndexOf(name)
	if index < 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "mcp server not found"})
		return
	}

	mcp.Loaded.Servers[index].Enabled = &enabled

	err = mcp.Save()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mcpReconnect()

	ctx.JSON(http.StatusOK, mcpEntry{
		Server: mcp.Loaded.Servers[index],
		Status: mcpStatusFor(mcp.StatusAll(), name),
	})
}

func McpList(ctx *gin.Context) {
	var all []mcp.Status
	var entry mcp.Server
	var out []mcpEntry

	var err error

	err = mcp.Load()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	all = mcp.StatusAll()

	for _, entry = range mcp.Loaded.Servers {
		out = append(out, mcpEntry{Server: entry, Status: mcpStatusFor(all, entry.Name)})
	}

	ctx.JSON(http.StatusOK, out)
}

func McpRead(ctx *gin.Context) {
	var name string
	var index int

	var err error

	name = ctx.Param("name")

	err = mcp.Load()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	index = mcpIndexOf(name)
	if index < 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "mcp server not found"})
		return
	}

	ctx.JSON(http.StatusOK, mcpEntry{
		Server: mcp.Loaded.Servers[index],
		Status: mcpStatusFor(mcp.StatusAll(), name),
	})
}

func McpCreate(ctx *gin.Context) {
	var req mcp.Server

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = mcp.Load()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if mcpIndexOf(req.Name) >= 0 {
		ctx.JSON(http.StatusConflict, gin.H{"error": "mcp server already exists"})
		return
	}

	err = mcp.Validate(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mcp.Loaded.Servers = append(mcp.Loaded.Servers, req)

	err = mcp.Save()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mcpReconnect()

	ctx.JSON(http.StatusCreated, mcpEntry{
		Server: req,
		Status: mcpStatusFor(mcp.StatusAll(), req.Name),
	})
}

func McpDelete(ctx *gin.Context) {
	var name string
	var index int

	var err error

	name = ctx.Param("name")

	err = mcp.Load()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	index = mcpIndexOf(name)
	if index < 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "mcp server not found"})
		return
	}

	mcp.Loaded.Servers = append(mcp.Loaded.Servers[:index], mcp.Loaded.Servers[index+1:]...)

	err = mcp.Save()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mcpReconnect()

	ctx.Status(http.StatusNoContent)
}

func McpEnable(ctx *gin.Context) {
	mcpSetEnabled(ctx, ctx.Param("name"), true)
}

func McpDisable(ctx *gin.Context) {
	mcpSetEnabled(ctx, ctx.Param("name"), false)
}

func McpReload(ctx *gin.Context) {
	var err error

	err = mcp.Reload(context.Background())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, mcp.StatusAll())
}
