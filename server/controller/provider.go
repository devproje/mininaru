// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type providerResponse struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	ApiKey  string `json:"api_key"`
	BaseUrl string `json:"base_url"`
	Active  bool   `json:"active"`
}

type providerCreateRequest struct {
	Name    string `json:"name" binding:"required"`
	ApiKey  string `json:"api_key"`
	BaseUrl string `json:"base_url"`
}

type providerUpdateRequest struct {
	Name    string `json:"name"`
	ApiKey  string `json:"api_key"`
	BaseUrl string `json:"base_url"`
}

func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}

	if len(secret) <= 8 {
		return "****"
	}

	return secret[:4] + "..." + secret[len(secret)-4:]
}

func toProviderResponse(prov *core.Provider) providerResponse {
	return providerResponse{
		Id:      prov.Id,
		Name:    prov.Name,
		ApiKey:  maskSecret(prov.ApiKey),
		BaseUrl: prov.BaseUrl,
		Active:  prov.Active,
	}
}

func ProviderCreate(ctx *gin.Context) {
	var req providerCreateRequest
	var prov core.Provider
	var created *core.Provider

	var err error

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	prov = core.Provider{Id: uuid.NewString(), Name: req.Name, ApiKey: req.ApiKey, BaseUrl: req.BaseUrl}

	err = core.ProviderCreate(&prov)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err = core.ProviderRead(prov.Id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, toProviderResponse(created))
}

func ProviderRead(ctx *gin.Context) {
	var id string
	var prov *core.Provider

	var err error

	id = ctx.Param("id")

	prov, err = core.ProviderRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, toProviderResponse(prov))
}

func ProviderList(ctx *gin.Context) {
	var list []*core.Provider
	var out []providerResponse
	var prov *core.Provider

	var err error

	list, err = core.ProviderList()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, prov = range list {
		out = append(out, toProviderResponse(prov))
	}

	ctx.JSON(http.StatusOK, out)
}

func ProviderUpdate(ctx *gin.Context) {
	var id string
	var req providerUpdateRequest
	var prov *core.Provider

	var err error

	id = ctx.Param("id")

	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = core.ProviderUpdate(id, &core.Provider{Name: req.Name, ApiKey: req.ApiKey, BaseUrl: req.BaseUrl})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	prov, err = core.ProviderRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, toProviderResponse(prov))
}

func ProviderDelete(ctx *gin.Context) {
	var id string

	var err error

	id = ctx.Param("id")

	err = core.ProviderDelete(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}

func ProviderActivate(ctx *gin.Context) {
	var id string
	var prov *core.Provider

	var err error

	id = ctx.Param("id")

	err = core.ProviderActivate(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	prov, err = core.ProviderRead(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, toProviderResponse(prov))
}
