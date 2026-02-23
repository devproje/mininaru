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
	"fmt"
	"net/url"
	"strings"

	"git.wh64.net/naru-studio/mininaru/log"
	"git.wh64.net/naru-studio/mininaru/modules/agent"
	"git.wh64.net/naru-studio/mininaru/modules/chat"
	"github.com/gin-gonic/gin"
)

func createEngine(ctx *gin.Context) {
	var err error
	var status int
	var message string

	var u *url.URL
	var body agent.AgentEngine
	var agent = WebServer.Agent

	err = ctx.BindJSON(&body)
	if err != nil {
		status = 400
		message = "body parse failed."

		goto err_cleanup
	}

	if body.Id == "" || body.Model == "" || body.ApiEndpoint == "" {
		status = 400
		message = "payload is not completed configured, please fill out your payload"

		goto err_cleanup
	}

	u, err = url.Parse(body.ApiEndpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		status = 400
		message = "api_endpoint is not url"

		goto err_cleanup
	}

	err = agent.CreateEngine(&body)
	if err != nil {
		status = 500
		message = "internal server error"

		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			status = 409
			message = fmt.Sprintf("engine id '%s' is already exists", body.Id)
		}

		goto err_cleanup
	}

	ctx.JSON(201, gin.H{
		"ok":      1,
		"message": "engine created",
		"data": gin.H{
			"id": body.Id,
		},
	})

	return

err_cleanup:
	if err != nil {
		log.Errorf("[webserver] unknown error occurred:\n%v\n", err)
	}

	ctx.JSON(status, gin.H{
		"ok":      0,
		"message": message,
	})
}

func createAgent(ctx *gin.Context) {
	var err error
	var status int
	var message string

	var body agent.AgentData
	var engine *agent.AgentEngine

	err = ctx.BindJSON(&body)
	if err != nil {
		status = 400
		message = "body parse failed"

		goto err_cleanup
	}

	engine, err = agent.Agent.ReadEngine(body.Engine.Id)
	if err != nil {
		status = 404
		message = fmt.Sprintf("engine id '%s' not exists", body.Engine.Id)

		goto err_cleanup
	}

	err = agent.Agent.Create(engine.Id, &body)
	if err != nil {
		status = 500
		message = "failed create agent"

		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			status = 409
			message = fmt.Sprintf("agent id '%s' is already exists", body.Id)
		}

		goto err_cleanup
	}

	ctx.JSON(200, gin.H{
		"ok":      1,
		"message": "agent created",
		"data": gin.H{
			"id":   body.Id,
			"name": body.Name,
		},
	})
	return

err_cleanup:
	if err != nil {
		log.Errorf("[webserver] unknown error occurred:\n%v\n", err)
	}

	ctx.JSON(status, gin.H{
		"ok":      0,
		"message": message,
	})
}

func send(ctx *gin.Context) {
	var err error
	var status int
	var message string

	var payload chat.ChatPayload
	var resp *chat.ChatResponse

	err = ctx.BindJSON(&payload)
	if err != nil {
		status = 400
		message = "body parse failed"

		goto err_cleanup
	}

	resp, err = chat.Chat.Send(&payload)
	if err != nil {
		status = 500
		message = "error occurred when sending message from llm provider"

		goto err_cleanup
	}

	ctx.JSON(200, gin.H{
		"ok":   1,
		"data": resp,
	})
	return

err_cleanup:
	if err != nil {
		log.Errorf("[webserver] unknown error occurred:\n%v\n", err)
	}

	ctx.JSON(status, gin.H{
		"ok":      0,
		"message": message,
	})
}

func routeV1(v1 *gin.RouterGroup) {
	var engine = v1.Group("/engine")
	var agent = v1.Group("/agent")
	var chat = v1.Group("/chat")

	engine.POST("", createEngine)
	agent.POST("", createAgent)
	chat.POST("", send)
}
