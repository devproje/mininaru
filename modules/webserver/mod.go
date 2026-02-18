// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * MiniNaru
 * Copyright (C) 2022-2026 Project_IO
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package webserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"git.wh64.net/naru-studio/mininaru/config"
	"git.wh64.net/naru-studio/mininaru/log"
	"git.wh64.net/naru-studio/mininaru/modules/agent"
	"github.com/gin-gonic/gin"
)

type WebServerModule struct {
	Agent     *agent.AgentModule
	Engine    *gin.Engine
	webserver *http.Server
	errc      chan error
}

func (m *WebServerModule) Name() string {
	return "webserver"
}

func (m *WebServerModule) Load() error {
	var err error
	var v1 *gin.RouterGroup

	m.Agent = agent.Agent

	m.Engine = gin.Default()
	m.webserver = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.Get.Host, config.Get.Port),
		Handler: m.Engine,
	}

	m.errc = make(chan error, 1)

	var proto = "http"
	if config.Get.SSL.Enable {
		proto = "https"
	}

	m.Engine.GET("/", index)
	v1 = m.Engine.Group("/v1")

	routeV1(v1)

	log.Printf("[webserver]: http webserver served at: %s://%s:%d\n", proto, config.Get.Host, config.Get.Port)

	go func() {
		if config.Get.SSL.Enable {
			m.errc <- m.webserver.ListenAndServeTLS(config.Get.SSL.CertFile, config.Get.SSL.KeyFile)
			return
		}

		m.errc <- m.webserver.ListenAndServe()
	}()

	select {
	case err = <-m.errc:
		return err
	case <-time.After(500 * time.Millisecond):
		break
	}

	return nil
}

func (m *WebServerModule) Unload() error {
	fmt.Printf("shutting down web server...\n")
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

	var err = m.webserver.Shutdown(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[webserver]: webserver forced to shutdown\n%v\n", err)
	}

	if m.Agent != nil {
		m.Agent = nil
	}

	cancel()
	return nil
}

var WebServer *WebServerModule = &WebServerModule{}
