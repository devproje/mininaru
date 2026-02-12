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
	"git.wh64.net/naru-studio/mininaru/modules/webserver/handler"
	"github.com/gin-gonic/gin"
)

type WebServerModule struct {
	Engine    *gin.Engine
	webserver *http.Server
}

func (m *WebServerModule) Name() string {
	return "webserver"
}

func (m *WebServerModule) Load() error {
	var err error
	m.Engine = gin.Default()
	m.webserver = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.Get.Host, config.Get.Port),
		Handler: m.Engine,
	}

	var proto = "http"
	if config.Get.SSL {
		proto = "https"
	}

	m.Engine.GET("/", handler.Index)

	log.Printf("[webserver]: http webserver served at: %s://%s:%d\n", proto, config.Get.Host, config.Get.Port)
	var errChannel = make(chan error, 1)

	go func() {
		if config.Get.SSL {
			// TODO: load ssl file
			errChannel <- m.webserver.ListenAndServeTLS("", "")
			return
		}

		errChannel <- m.webserver.ListenAndServe()
	}()

	select {
	case err = <-errChannel:
		return err
	case <-time.After(500 * time.Millisecond):
		break
	}

	return nil
}

func (m *WebServerModule) Unload() error {
	fmt.Printf("shutting down web server...\n")
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = m.webserver.Shutdown(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[WebServer] webserver forced to shutdown %v\n", err)
	}

	return nil
}

var WebServer *WebServerModule = &WebServerModule{}
