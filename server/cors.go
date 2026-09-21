// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

func corsMiddleware(origins []string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var origin string

		origin = ctx.GetHeader("Origin")
		if origin == "" || !slices.Contains(origins, origin) {
			ctx.Next()
			return
		}

		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Vary", "Origin")

		if ctx.Request.Method != http.MethodOptions {
			ctx.Next()
			return
		}

		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		ctx.Header("Access-Control-Max-Age", "600")
		ctx.AbortWithStatus(http.StatusNoContent)
	}
}
