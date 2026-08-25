// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"testing"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
	"github.com/gin-gonic/gin"
)

func setupTestDB(t *testing.T) {
	var err error

	t.Helper()

	gin.SetMode(gin.TestMode)

	err = util.InitFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.DB, err = util.NewDatabase(util.Path("data.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		util.DB.Close()
	})
}

func newRouter() *gin.Engine {
	var router *gin.Engine
	var api *gin.RouterGroup
	var v1 *gin.RouterGroup
	var agents *gin.RouterGroup
	var providers *gin.RouterGroup
	var sessions *gin.RouterGroup
	var messages *gin.RouterGroup

	router = gin.New()
	api = router.Group("/api")
	v1 = router.Group("/api/v1")

	agents = api.Group("/agents")
	agents.POST("", AgentCreate)
	agents.GET("", AgentList)
	agents.GET("/:id", AgentRead)
	agents.PATCH("/:id", AgentUpdate)
	agents.DELETE("/:id", AgentDelete)

	providers = api.Group("/providers")
	providers.POST("", ProviderCreate)
	providers.GET("", ProviderList)
	providers.GET("/:id", ProviderRead)
	providers.PATCH("/:id", ProviderUpdate)
	providers.DELETE("/:id", ProviderDelete)
	providers.POST("/:id/activate", ProviderActivate)

	sessions = api.Group("/sessions")
	sessions.POST("", SessionCreate)
	sessions.GET("", SessionList)
	sessions.GET("/:id", SessionRead)
	sessions.PATCH("/:id", SessionUpdate)
	sessions.DELETE("/:id", SessionDelete)
	sessions.POST("/:id/messages", MessageCreate)
	sessions.GET("/:id/messages", MessageList)

	messages = api.Group("/messages")
	messages.GET("/:id", MessageRead)
	messages.PATCH("/:id", MessageUpdate)
	messages.DELETE("/:id", MessageDelete)

	v1.POST("/chat/completions", ChatCompletions)
	v1.GET("/models", Models)

	return router
}

func createTestAgent(t *testing.T) string {
	var err error

	t.Helper()

	err = core.AgentCreate(&core.Agent{Id: "a1", Name: "naru", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}

	return "a1"
}

func createTestSession(t *testing.T) (string, string) {
	var agentId string
	var err error

	t.Helper()

	agentId = createTestAgent(t)

	err = core.SessionCreate(&core.Session{Id: "s1", AgentId: agentId})
	if err != nil {
		t.Fatal(err)
	}

	return agentId, "s1"
}
