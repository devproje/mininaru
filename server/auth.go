// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func authMiddleware(key string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var header string
		var token string

		header = ctx.GetHeader("Authorization")
		token = strings.TrimPrefix(header, "Bearer ")

		if token == "" || token != key {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx.Next()
	}
}
