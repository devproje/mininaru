package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"time"

	"git.wh64.net/naru-studio/mininaru/config"
	"github.com/gin-gonic/gin"
)

var NaruCore *MiniNaru

type MiniNaru struct {
	webserver *http.Server
	orders    []string
	modules   map[string]NaruModule

	Initialzed bool
	Engine     *gin.Engine
}

func NewMiniNaru() *MiniNaru {
	return &MiniNaru{}
}

func (n *MiniNaru) Insmod(module NaruModule) {
	if n.Initialzed {
		_, _ = fmt.Fprintf(os.Stderr, "your module is ignored by this system, because service already loaded.\n")
		return
	}

	n.modules[module.Name()] = module
}

func (n *MiniNaru) Init() error {
	if n.Initialzed {
		return fmt.Errorf("mininaru core is already loaded")
	}

	var err error
	var proto = "http"
	if config.Get.SSL {
		proto = "https"
	}

	var ver = config.Get.Ver

	fmt.Println()
	fmt.Println("███╗   ██╗ █████╗ ██████╗ ██╗   ██╗")
	fmt.Println("████╗  ██║██╔══██╗██╔══██╗██║   ██║")
	fmt.Println("██╔██╗ ██║███████║██████╔╝██║   ██║")
	fmt.Println("██║╚██╗██║██╔══██║██╔══██╗██║   ██║")
	fmt.Println("██║ ╚████║██║  ██║██║  ██║╚██████╔╝")
	fmt.Println("╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝")
	fmt.Println()

	fmt.Printf("starting mininaru v%s-%s (%s)\n", ver.Version, ver.Branch, ver.GitHash)

	for name, module := range n.modules {
		fmt.Printf("loading naru module: %s\n", name)
		err = module.Load()
		if err != nil {
			return fmt.Errorf("failed to load module %s: %v", name, err)
		}

		n.orders = append(n.orders, name)
	}

	n.Engine = gin.Default()
	n.webserver = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.Get.Host, config.Get.Port),
		Handler: n.Engine,
	}
	n.Initialzed = true

	fmt.Printf("http webserver served at: %s://%s:%d\n", proto, config.Get.Host, config.Get.Port)
	go func() {
		if config.Get.SSL {
			// TODO: load ssl file
			var err = n.webserver.ListenAndServeTLS("", "")
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "failed load webserver with tls: %v\n", err)
			}

			return
		}

		var err = n.webserver.ListenAndServe()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed load webserver: %v\n", err)
		}
	}()

	return nil
}

func (n *MiniNaru) Destroy() error {
	if !n.Initialzed {
		return fmt.Errorf("mininaru core's state is already dead")
	}

	fmt.Printf("shutting down web server...\n")
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err = n.webserver.Shutdown(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "webserver forced to shutdown %v\n", err)
	}

	slices.Reverse(n.orders)
	for _, order := range n.orders {
		fmt.Printf("unloading naru module: %s\n", order)
		var module, ok = n.modules[order]
		if !ok {
			continue
		}

		err = module.Unload()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to unload module %s: %v\n", order, err)
		}
	}

	n.Initialzed = false
	n.orders = nil

	return nil
}
