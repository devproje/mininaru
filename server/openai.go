// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"github.com/devproje/mininaru/server/controller"
	"github.com/gin-gonic/gin"
)

func openAIRoutes(v1 *gin.RouterGroup) {
	v1.POST("/chat/completions", controller.ChatCompletions)
	v1.GET("/models", controller.Models)
}
