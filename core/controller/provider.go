// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

type providerCreateBody struct {
	Name    string `json:"name"`
	ApiKey  string `json:"api_key,omitempty"`
	BaseUrl string `json:"base_url,omitempty"`
}

type providerUpdateBody struct {
	ApiKey  string `json:"api_key,omitempty"`
	BaseUrl string `json:"base_url,omitempty"`
}

func addProvider(ctx *gin.Context) {
	var body providerCreateBody
	var id string

	var err error

	err = ctx.ShouldBindBodyWithJSON(&body)
	if err != nil {
		ctx.JSON(400, gin.H{
			"ok":    0,
			"error": "invalid payload",
		})

		return
	}

	id, err = AddProvider(context.Background(), body.Name, body.ApiKey, body.BaseUrl)
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))
		ctx.JSON(500, gin.H{
			"ok":    0,
			"error": "transaction failed",
		})

		return
	}

	ctx.JSON(200, gin.H{
		"ok": 1,
		"id": id,
	})
}

func getProviders(ctx *gin.Context) {
	var objs []Provider

	var err error

	objs, err = GetProviders()
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(500, gin.H{
				"ok":    0,
				"error": "database error",
			})

			return
		}

		ctx.JSON(200, gin.H{
			"ok":   1,
			"data": objs,
		})
	}

	ctx.JSON(200, gin.H{
		"ok":   1,
		"data": objs,
	})
}

func getProvider(ctx *gin.Context) {
	var id string
	var obj Provider

	var err error

	id = ctx.Param("id")

	obj, err = GetProvider(id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(500, gin.H{
				"ok":    0,
				"error": "database error",
			})

			return
		}

		ctx.JSON(204, gin.H{
			"ok":   0,
			"data": nil,
		})

		return
	}

	ctx.JSON(200, gin.H{
		"ok":   1,
		"data": obj,
	})
}

func setProvider(ctx *gin.Context) {
	var id string
	var body providerUpdateBody
	var obj Provider

	var err error

	id = ctx.Param("id")
	err = ctx.ShouldBindBodyWithJSON(&body)
	if err != nil {
		ctx.JSON(400, gin.H{
			"ok":    0,
			"error": "invalid payload",
		})

		return
	}

	obj = Provider{
		ApiKey:  body.ApiKey,
		BaseUrl: body.BaseUrl,
	}

	err = SetProvider(context.Background(), id, &obj)
	if err != nil {
		if !errors.Is(err, ErrNoFieldsToUpdate) {
			ctx.JSON(500, gin.H{
				"ok":    0,
				"error": "transaction error",
			})

			return
		}

		ctx.JSON(400, gin.H{
			"ok":    0,
			"error": err.Error(),
		})

		return
	}

	ctx.JSON(200, gin.H{
		"ok": 1,
		"id": id,
	})
}

func emptyApiKey(ctx *gin.Context) {
	// TODO
}

func emptyBaseUrl(ctx *gin.Context) {
	// TODO
}

func removeProvider(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")
	err = RemoveProvider(context.Background(), id)
	if err != nil {
		ctx.JSON(500, gin.H{
			"ok":    0,
			"error": "transaction error",
		})

		return
	}

	ctx.JSON(200, gin.H{
		"ok": 1,
		"id": id,
	})
}

func RouteProviders(api *gin.RouterGroup) {
	api.POST("/providers", addProvider)
	api.GET("/providers", getProviders)
	api.GET("/providers/:id", getProvider)
	api.PUT("/providers/:id", setProvider)
	api.PATCH("/providers/:id/apikey", emptyApiKey)
	api.PATCH("/providers/:id/baseurl", emptyBaseUrl)
	api.DELETE("/providers/:id", removeProvider)
}
