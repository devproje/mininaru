// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package controller

import (
	"errors"
	"fmt"
	"os"

	"github.com/devproje/mininaru/modules/store"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

type jsonProfile struct {
	Name  *string `json:"name,omitempty"`
	Model *string `json:"model,omitempty"`

	Soul *string `json:"soul,omitempty"`
}

func upsertProfile(ctx *gin.Context) {
	var body jsonProfile
	var obj store.Profile

	var err error

	err = ctx.ShouldBindBodyWithJSON(&body)
	if err != nil {
		ctx.JSON(400, errInvalidPayload)
		return
	}

	obj = store.Profile{
		Name:  body.Name,
		Model: body.Model,
		Soul:  body.Soul,
	}

	err = store.UpsertProfile(&obj)
	if errors.Is(err, store.ErrModelNotFound) {
		ctx.JSON(400, errInvalidProfileModel)
		return
	}
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from profile: %v", err))
		ctx.JSON(500, errProfileSave)

		return
	}

	ctx.JSON(200, gin.H{
		"ok":     1,
		"action": "UPSERT_PROFILE",
	})
}

func getProfile(ctx *gin.Context) {
	var raw store.Profile
	var obj jsonProfile

	var err error

	raw, err = store.GetProfile()
	if errors.Is(err, os.ErrNotExist) {
		ctx.JSON(404, errProfileNotInitialized)
		return
	}
	if err != nil {
		util.Log.Error(fmt.Sprintf("sent error message from profile: %v", err))
		ctx.JSON(500, errProfileRead)

		return
	}

	obj = jsonProfile{
		Name:  raw.Name,
		Model: raw.Model,
		Soul:  raw.Soul,
	}

	ctx.JSON(200, obj)
}

func RouteProfile(api *gin.RouterGroup) {
	api.POST("/profile", upsertProfile)
	api.GET("/profile", getProfile)
}
