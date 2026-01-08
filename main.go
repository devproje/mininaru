package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"git.wh64.net/naru-studio/mininaru/config"
	"git.wh64.net/naru-studio/mininaru/core"
	"git.wh64.net/naru-studio/mininaru/handler"
	"github.com/gin-gonic/gin"
)

var (
	version   string
	branch    string
	hash      string
	buildTime string
	goVersion string
)

func main() {
	var quit chan os.Signal
	core.NaruCore = core.NewMiniNaru()
	core.NaruCore.Modules = make(map[string]core.NaruModule, 0)

	config.Load(&config.VersionInfo{
		Version:   version,
		Branch:    branch,
		GitHash:   hash,
		BuildTime: buildTime,
		GoVersion: goVersion,
	})

	gin.SetMode(gin.ReleaseMode)
	for _, arg := range os.Args {
		if arg == "--debug" || arg == "-d" {
			gin.SetMode(gin.DebugMode)
		}
	}

	var err = core.NaruCore.Init()
	var app = core.NaruCore.Engine
	if err != nil {
		goto cleanup
	}

	app.GET("/", handler.Index)

	quit = make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	goto cleanup

cleanup:
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	if !core.NaruCore.Initialzed {
		return
	}

	core.NaruCore.Destroy()
}
