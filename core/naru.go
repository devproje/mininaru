package core

import (
	"fmt"
	"os"
	"slices"
	"sync"

	"git.wh64.net/naru-studio/mininaru/config"
)

var NaruCore *MiniNaru

type NaruModule interface {
	Name() string

	Load() error
	Unload() error
}

type MiniNaru struct {
	sync.RWMutex
	orders  []string
	modules map[string]NaruModule

	Initialized bool
}

func NewMiniNaru() *MiniNaru {
	return &MiniNaru{}
}

func (n *MiniNaru) Insmod(module NaruModule) {
	n.Lock()
	defer n.Unlock()

	if n.Initialized {
		_, _ = fmt.Fprintf(os.Stderr, "your module is ignored by this system, because service already loaded.\n")
		return
	}

	if n.modules == nil {
		n.modules = make(map[string]NaruModule, 0)
	}

	n.modules[module.Name()] = module
	n.orders = append(n.orders, module.Name())
}

func (n *MiniNaru) Init() error {
	if n.Initialized {
		return fmt.Errorf("mininaru core is already loaded")
	}

	var err error
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

	n.Lock()
	defer n.Unlock()

	for _, name := range n.orders {
		var module = n.modules[name]
		fmt.Printf("loading naru module: %s\n", name)

		err = module.Load()
		if err != nil {
			return fmt.Errorf("failed to load module %s: %v", name, err)
		}
	}

	n.Initialized = true

	fmt.Printf("mininaru core is ready.\n")
	return nil
}

func (n *MiniNaru) Destroy() error {
	var err error
	if !n.Initialized {
		return fmt.Errorf("mininaru core's state is already dead")
	}

	slices.Reverse(n.orders)

	n.Lock()
	defer n.Unlock()
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

	n.Initialized = false
	n.orders = nil

	return nil
}
