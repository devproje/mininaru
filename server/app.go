// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/devproje/mininaru/server/sock"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

type Options struct {
	Host        string
	Port        uint16
	ApiKey      string
	CorsOrigins []string
	WebDir      string
}

type AppServer struct {
	WebServer *http.Server
}

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
)

var App *AppServer

func NewAppServer(options Options) *AppServer {
	var core *gin.Engine
	var api *gin.RouterGroup
	var v1 *gin.RouterGroup
	var webserver http.Server

	var app AppServer

	if !util.AppDebug {
		gin.SetMode(gin.ReleaseMode)
	}

	core = gin.Default()
	core.Use(corsMiddleware(options.CorsOrigins))

	api = core.Group("/api", authMiddleware(options.ApiKey))
	v1 = core.Group("/api/v1", authMiddleware(options.ApiKey))

	apiRoutes(api)
	openAIRoutes(v1)

	core.GET("/ws", authMiddleware(options.ApiKey), sock.SockHandler)

	if options.WebDir != "" {
		core.NoRoute(webMiddleware(options.WebDir))
	}

	webserver = http.Server{
		Addr:              fmt.Sprintf("%s:%d", options.Host, options.Port),
		Handler:           core,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}

	app = AppServer{
		WebServer: &webserver,
	}

	return &app
}
