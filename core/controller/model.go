// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"context"
	"fmt"

	"github.com/devproje/mininaru/modules/store"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

func listModel(ctx *gin.Context) {
	var models []string

	var err error

	models, err = store.ListModel(context.Background())
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from model: %v", err))
		ctx.JSON(500, errModelList)

		return
	}

	ctx.JSON(200, models)
}

func validateModel(ctx *gin.Context) {
	var model string
	var ok bool

	var err error

	model = ctx.Param("model")
	ok, err = store.ValidateModel(context.Background(), model)
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from model validation: %v", err))
		ctx.JSON(500, errModelValidation)

		return
	}
	if !ok {
		ctx.JSON(404, errorTemplate(fmt.Sprintf("model %q is not listed by configured providers", model)))
		return
	}

	ctx.JSON(200, gin.H{
		"ok":     1,
		"action": "VALIDATE_MODEL",
	})
}

func RouteModels(v1, api *gin.RouterGroup) {
	v1.GET("/models", listModel)
	api.GET("/models/validate/:model", validateModel)
}
