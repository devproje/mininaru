// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func isReserved(path string) bool {
	return path == "/api" || path == "/ws" || strings.HasPrefix(path, "/api/")
}

func isRegularFile(fs http.FileSystem, name string) bool {
	var file http.File
	var info os.FileInfo

	var err error

	file, err = fs.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()

	info, err = file.Stat()
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func webMiddleware(dir string) gin.HandlerFunc {
	var fs http.FileSystem

	fs = http.Dir(dir)

	return func(ctx *gin.Context) {
		var name string
		var index []byte

		var err error

		name = ctx.Request.URL.Path

		if isReserved(name) || (ctx.Request.Method != http.MethodGet && ctx.Request.Method != http.MethodHead) {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}

		if name != "/" && !strings.HasSuffix(name, "/index.html") && isRegularFile(fs, name) {
			ctx.FileFromFS(name, fs)
			return
		}

		index, err = os.ReadFile(filepath.Join(dir, "index.html"))
		if err != nil {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}

		ctx.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
