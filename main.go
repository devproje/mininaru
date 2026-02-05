package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"git.wh64.net/naru-studio/mininaru/config"
	"git.wh64.net/naru-studio/mininaru/core"
	"git.wh64.net/naru-studio/mininaru/modules/llm"
	"git.wh64.net/naru-studio/mininaru/modules/webserver"
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

	var err = config.Load(&config.VersionInfo{
		Version:   version,
		Branch:    branch,
		GitHash:   hash,
		BuildTime: buildTime,
		GoVersion: goVersion,
	})
	if err != nil {
		goto cleanup
	}

	gin.SetMode(gin.ReleaseMode)
	for _, arg := range os.Args {
		if arg == "--debug" || arg == "-d" {
			gin.SetMode(gin.DebugMode)
		}
	}

	core.NaruCore.Insmod(webserver.WebServer)
	core.NaruCore.Insmod(llm.LLM)

	err = core.NaruCore.Init()
	if err != nil {
		goto cleanup
	}

	quit = make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	goto cleanup

cleanup:
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	if !core.NaruCore.Initialized {
		return
	}

	core.NaruCore.Destroy()
}
