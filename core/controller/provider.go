// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/devproje/mininaru/modules/store"
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
		ctx.JSON(400, errInvalidPayload)
		return
	}

	id, err = store.AddProvider(context.Background(), body.Name, body.ApiKey, body.BaseUrl)
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))
		ctx.JSON(500, errProviderCreate)

		return
	}

	ctx.JSON(200, gin.H{
		"ok":     1,
		"id":     id,
		"action": "CREATE_PROVIDER",
	})
}

func getProviders(ctx *gin.Context) {
	var objs []store.Provider

	var err error

	objs, err = store.GetProviders()
	if errors.Is(err, sql.ErrNoRows) {
		ctx.JSON(200, gin.H{
			"ok":   1,
			"data": objs,
		})

		return
	}
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))
		ctx.JSON(500, errProviderList)

		return
	}

	ctx.JSON(200, objs)
}

func getProvider(ctx *gin.Context) {
	var id string
	var obj store.Provider

	var err error

	id = ctx.Param("id")
	obj, err = store.GetProvider(id)
	if errors.Is(err, sql.ErrNoRows) {
		ctx.JSON(404, errProviderNotFound)
		return
	}
	if err != nil {
		ctx.JSON(500, errProviderRead)
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))

		return
	}

	ctx.JSON(200, obj)
}

func setProvider(ctx *gin.Context) {
	var id string
	var body providerUpdateBody
	var obj store.Provider

	var err error

	id = ctx.Param("id")
	err = ctx.ShouldBindBodyWithJSON(&body)
	if err != nil {
		ctx.JSON(400, errInvalidPayload)
		return
	}

	obj = store.Provider{
		ApiKey:  body.ApiKey,
		BaseUrl: body.BaseUrl,
	}

	err = store.SetProvider(context.Background(), id, &obj)
	if errors.Is(err, store.ErrNoFieldsToUpdate) {
		ctx.JSON(400, errorTemplate(err.Error()))
		return
	}
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))
		ctx.JSON(500, errProviderUpdate)

		return
	}

	ctx.JSON(200, gin.H{
		"ok":     1,
		"id":     id,
		"action": "UPDATE_PROVIDER",
	})
}

func emptyApiKey(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")
	err = store.SetEmptyApiKey(context.Background(), id)
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))
		ctx.JSON(500, errProviderUpdate)

		return
	}

	ctx.JSON(200, gin.H{
		"ok":     1,
		"id":     id,
		"action": "EMPTY_API_KEY",
	})
}

func emptyBaseUrl(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")
	err = store.SetEmptyBaseUrl(context.Background(), id)
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))
		ctx.JSON(500, errProviderUpdate)

		return
	}

	ctx.JSON(200, gin.H{
		"ok":     1,
		"id":     id,
		"action": "EMPTY_BASE_URL",
	})
}

func removeProvider(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")
	err = store.RemoveProvider(context.Background(), id)
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from provider: %v", err))
		ctx.JSON(500, errProviderDelete)

		return
	}

	ctx.JSON(200, gin.H{
		"ok":     1,
		"id":     id,
		"action": "DELETE_PROVIDER",
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
