// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"net/http"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/modules/skill"
	"github.com/gin-gonic/gin"
)

type skillSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Scope       string `json:"scope"`
}

func SkillList(ctx *gin.Context) {
	var item skill.Skill
	var out []skillSummary

	for _, item = range skill.All() {
		out = append(out, skillSummary{Name: item.Name, Description: item.Description, Path: item.Path, Scope: item.Scope})
	}

	ctx.JSON(http.StatusOK, out)
}

func SkillRead(ctx *gin.Context) {
	var name string
	var text string

	var err error

	name = ctx.Param("name")

	if skill.Find(name) == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	text, err = skill.Result(name, "")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"name": name, "text": text})
}

func SkillUses(ctx *gin.Context) {
	var uses []*core.SkillUse

	var err error

	uses, err = core.SkillUseStats(ctx.Query("session"))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, uses)
}
