// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import "github.com/gin-gonic/gin"

func addAgent(ctx *gin.Context) {}

func getAgents(ctx *gin.Context) {}

func getAgent(ctx *gin.Context) {}

func setAgent(ctx *gin.Context) {}

func removeAgent(ctx *gin.Context) {}

func RouteAgents(api *gin.RouterGroup) {
	api.POST("/agents", addAgent)
	api.GET("/agents", getAgents)
	api.GET("/agents/:id", getAgent)
	api.PUT("/agents/:id", setAgent)
	api.DELETE("/agents/:id", removeAgent)
}
