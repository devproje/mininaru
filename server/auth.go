// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"net/http"
	"strings"

	"github.com/devproje/mininaru/server/sock"
	"github.com/gin-gonic/gin"
)

func subprotocolToken(ctx *gin.Context) string {
	var protocol string

	if !ctx.IsWebsocket() {
		return ""
	}

	for _, protocol = range strings.Split(ctx.GetHeader("Sec-WebSocket-Protocol"), ",") {
		protocol = strings.TrimSpace(protocol)
		if strings.HasPrefix(protocol, sock.SubprotocolPrefix) {
			return strings.TrimPrefix(protocol, sock.SubprotocolPrefix)
		}
	}

	return ""
}

func requestToken(ctx *gin.Context) string {
	var header string

	header = ctx.GetHeader("Authorization")
	if header != "" {
		return strings.TrimPrefix(header, "Bearer ")
	}

	return subprotocolToken(ctx)
}

func authMiddleware(key string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var token string

		token = requestToken(ctx)

		if token == "" || token != key {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx.Next()
	}
}
