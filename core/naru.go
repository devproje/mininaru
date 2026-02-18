// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * MiniNaru
 * Copyright (C) 2022-2026 Project_IO
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package core

import (
	"fmt"
	"os"
	"slices"
	"sync"

	"git.wh64.net/naru-studio/mininaru/config"
	"git.wh64.net/naru-studio/mininaru/log"
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

	if n.Initialized {
		_, _ = fmt.Fprintf(os.Stderr, "your module is ignored by this system, because service already loaded.\n")
		n.Unlock()

		return
	}

	if n.modules == nil {
		n.modules = make(map[string]NaruModule, 0)
	}

	n.modules[module.Name()] = module
	n.orders = append(n.orders, module.Name())

	n.Unlock()
}

func (n *MiniNaru) Init() error {
	var err error
	var ver = config.Get.Ver

	if n.Initialized {
		err = fmt.Errorf("[mininaru] mininaru core is already loaded")
		goto err_cleanup
	}

	err = log.Init()
	if err != nil {
		goto err_cleanup
	}

	fmt.Println()
	fmt.Println("███╗   ██╗ █████╗ ██████╗ ██╗   ██╗")
	fmt.Println("████╗  ██║██╔══██╗██╔══██╗██║   ██║")
	fmt.Println("██╔██╗ ██║███████║██████╔╝██║   ██║")
	fmt.Println("██║╚██╗██║██╔══██║██╔══██╗██║   ██║")
	fmt.Println("██║ ╚████║██║  ██║██║  ██║╚██████╔╝")
	fmt.Println("╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝")
	fmt.Println()

	log.Printf("[mininaru]: starting mininaru %s-%s (%s)\n", ver.Version, ver.Branch, ver.GitHash)

	n.Lock()

	for _, name := range n.orders {
		var module = n.modules[name]
		log.Printf("[mininaru]: loading naru module: %s\n", name)

		err = module.Load()
		if err != nil {
			err = fmt.Errorf("[mininaru]: failed to load module %s: %v", name, err)
			goto err_module_cleanup
		}
	}

	n.Unlock()
	n.Initialized = true

	log.Printf("[mininaru]: mininaru core is ready.\n")
	return nil

err_module_cleanup:
	n.Unlock()
err_cleanup:
	return err
}

func (n *MiniNaru) Destroy() error {
	var err error
	if !n.Initialized {
		return fmt.Errorf("[mininaru]: mininaru core's state is already dead")
	}

	slices.Reverse(n.orders)
	n.Lock()

	for _, order := range n.orders {
		log.Printf("[mininaru]: unloading naru module: %s\n", order)
		var module, ok = n.modules[order]
		if !ok {
			continue
		}

		err = module.Unload()
		if err != nil {
			_, _ = log.Errorf("[mininaru]: failed to unload module %s: %v\n", order, err)
		}
	}

	n.Unlock()
	n.orders = nil

	_ = log.Destroy()
	n.Initialized = false

	return nil
}
