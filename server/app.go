// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"fmt"
	"net/http"

	"github.com/devproje/mininaru/server/sock"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

type AppServer struct {
	WebServer *http.Server
}

var App *AppServer

func NewAppServer(host string, port uint16, apiKey string) *AppServer {
	var core *gin.Engine
	var webserver http.Server

	var api *gin.RouterGroup
	var v1 *gin.RouterGroup

	var app AppServer

	if !util.AppDebug {
		gin.SetMode(gin.ReleaseMode)
	}

	core = gin.Default()

	api = core.Group("/api", authMiddleware(apiKey))
	v1 = core.Group("/api/v1", authMiddleware(apiKey))

	apiRoutes(api)
	openAIRoutes(v1)

	core.GET("/ws", authMiddleware(apiKey), sock.SockHandler)

	webserver = http.Server{
		Addr:    fmt.Sprintf("%s:%d", host, port),
		Handler: core,
	}

	app = AppServer{
		WebServer: &webserver,
	}

	return &app
}
