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

	Initialzed bool
	Engine     *gin.Engine
	Modules    map[string]NaruModule
}

func NewMiniNaru() *MiniNaru {
	return &MiniNaru{}
}

func (n *MiniNaru) Init() error {
	if n.Initialzed {
		return fmt.Errorf("mininaru core is already loaded")
	}

	fmt.Println()
	fmt.Println("███╗   ██╗ █████╗ ██████╗ ██╗   ██╗")
	fmt.Println("████╗  ██║██╔══██╗██╔══██╗██║   ██║")
	fmt.Println("██╔██╗ ██║███████║██████╔╝██║   ██║")
	fmt.Println("██║╚██╗██║██╔══██║██╔══██╗██║   ██║")
	fmt.Println("██║ ╚████║██║  ██║██║  ██║╚██████╔╝")
	fmt.Println("╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝")
	fmt.Println()

	var ver = config.Get.Ver
	fmt.Printf("starting mininaru v%s-%s (%s)\n", ver.Version, ver.Branch, ver.GitHash)

	for name, module := range n.Modules {
		fmt.Printf("loading naru module: %s\n", name)
		var err = module.Load()
		if err != nil {
			return fmt.Errorf("failed to load module %s: %v", name, err)
		}

		n.orders = append(n.orders, name)
	}

	n.Engine = gin.Default()
	n.webserver = &http.Server{
		Addr:    ":3000",
		Handler: n.Engine,
	}
	n.Initialzed = true

	go func() {
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
		var module, ok = n.Modules[order]
		if !ok {
			continue
		}

		var err = module.Unload()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to unload module %s: %v\n", order, err)
		}
	}

	n.Initialzed = false
	n.orders = nil

	return nil
}
