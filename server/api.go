// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"github.com/devproje/mininaru/server/controller"
	"github.com/gin-gonic/gin"
)

func apiRoutes(api *gin.RouterGroup) {
	var sessions *gin.RouterGroup
	var messages *gin.RouterGroup
	var agents *gin.RouterGroup
	var providers *gin.RouterGroup

	agents = api.Group("/agents")
	agents.POST("", controller.AgentCreate)
	agents.GET("", controller.AgentList)
	agents.GET("/:id", controller.AgentRead)
	agents.PATCH("/:id", controller.AgentUpdate)
	agents.DELETE("/:id", controller.AgentDelete)

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
}
