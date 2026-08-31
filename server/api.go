// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"github.com/devproje/mininaru/server/controller"
	"github.com/gin-gonic/gin"
)

func apiRoutes(api *gin.RouterGroup) {
	var agents *gin.RouterGroup
	var providers *gin.RouterGroup
	var sessions *gin.RouterGroup
	var messages *gin.RouterGroup
	var mcpServers *gin.RouterGroup
	var skills *gin.RouterGroup

	agents = api.Group("/agents")
	agents.POST("", controller.AgentCreate)
	agents.GET("", controller.AgentList)
	agents.GET("/:id", controller.AgentRead)
	agents.PATCH("/:id", controller.AgentUpdate)
	agents.DELETE("/:id", controller.AgentDelete)
	agents.GET("/:id/memory", controller.MemoryList)
	agents.GET("/:id/memory/:file", controller.MemoryRead)
	agents.PUT("/:id/memory/:file", controller.MemoryWrite)
	agents.DELETE("/:id/memory/:file", controller.MemoryDelete)

	providers = api.Group("/providers")
	providers.POST("", controller.ProviderCreate)
	providers.GET("", controller.ProviderList)
	providers.GET("/:id", controller.ProviderRead)
	providers.PATCH("/:id", controller.ProviderUpdate)
	providers.DELETE("/:id", controller.ProviderDelete)
	providers.POST("/:id/activate", controller.ProviderActivate)

	sessions = api.Group("/sessions")
	sessions.POST("", controller.SessionCreate)
	sessions.GET("", controller.SessionList)
	sessions.GET("/:id", controller.SessionRead)
	sessions.PATCH("/:id", controller.SessionUpdate)
	sessions.DELETE("/:id", controller.SessionDelete)
	sessions.POST("/:id/messages", controller.MessageCreate)
	sessions.GET("/:id/messages", controller.MessageList)

	messages = api.Group("/messages")
	messages.GET("/:id", controller.MessageRead)
	messages.PATCH("/:id", controller.MessageUpdate)
	messages.DELETE("/:id", controller.MessageDelete)

	mcpServers = api.Group("/mcp")
	mcpServers.GET("", controller.McpList)
	mcpServers.POST("", controller.McpCreate)
	mcpServers.POST("/reload", controller.McpReload)
	mcpServers.GET("/:name", controller.McpRead)
	mcpServers.DELETE("/:name", controller.McpDelete)
	mcpServers.POST("/:name/enable", controller.McpEnable)
	mcpServers.POST("/:name/disable", controller.McpDisable)

	skills = api.Group("/skill")
	skills.GET("", controller.SkillList)
	skills.GET("/uses", controller.SkillUses)
	skills.GET("/:name", controller.SkillRead)

	api.POST("/yolo", controller.YoloSet)
	api.GET("/yolo", controller.YoloGet)
}
