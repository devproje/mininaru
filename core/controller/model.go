// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
)

func listModel(ctx *gin.Context) {
	var models []string

	var err error

	models, err = ListModel(context.Background())
	if err != nil {
		ctx.JSON(500, gin.H{
			"ok":    0,
			"error": "failed fetch model",
		})

		return
	}

	ctx.JSON(200, models)
}

func validateModel(ctx *gin.Context) {
	var model string
	var ok bool

	var err error

	model = ctx.Param("model")

	ok, err = ValidateModel(context.Background(), model)
	if err != nil {
		ctx.JSON(500, gin.H{
			"ok":    0,
			"error": "failed fetch model",
		})

		return
	}

	if !ok {
		ctx.JSON(400, gin.H{
			"ok":    0,
			"error": fmt.Sprintf("model '%s' not exists", model),
		})

		return
	}

	ctx.JSON(200, gin.H{
		"ok": 1,
	})
}

func RouteModels(v1, api *gin.RouterGroup) {
	v1.GET("/models", listModel)
	api.GET("/model/validate/:model", validateModel)
}
