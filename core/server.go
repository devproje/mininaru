// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"github.com/devproje/mininaru/core/controller"
	"github.com/devproje/mininaru/core/sock"
	"github.com/gin-gonic/gin"
)

type NaruCore struct {
	App *gin.Engine
}

func NewNaruCore() *NaruCore {
	var app *gin.Engine
	var v1 *gin.RouterGroup
	var api *gin.RouterGroup

	app = gin.Default()

	v1 = app.Group("/v1")
	api = app.Group("/api")

	controller.RouteProviders(api)
	controller.RouteModels(v1, api)

	app.GET("/ws", sock.Handler)

	return &NaruCore{
		App: app,
	}
}
