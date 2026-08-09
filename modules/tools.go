// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
)

type Permission int

type Def struct {
	Name        string
	Description string
	Parameters  map[string]any
	Permission  Permission
	Execute     func(ctx context.Context, arguments string) (string, error)
	daemon      bool
}

const (
	PermissionSafe Permission = iota
	PermissionDangerous
	PermissionPrivileged
)

var workingRoot string

func (p Permission) String() string {
	switch p {
	case PermissionDangerous:
		return "dangerous"
	case PermissionPrivileged:
		return "privileged"
	default:
		return "safe"
	}
}

func SetWorkingRoot(root string) error {
	var resolved string

	var err error

	resolved, err = toolRoot(root)
	if err != nil {
		return err
	}

	workingRoot = resolved
	return nil
}

func DefaultTools() []Def {
	var tools []Def

	tools = append(tools, builtinDefs()...)
	tools = append(tools, manager.defs()...)

	return tools
}

func SafeTools() []Def {
	var def Def
	var tools []Def

	for _, def = range DefaultTools() {
		if def.Permission != PermissionSafe || !def.daemon {
			continue
		}

		tools = append(tools, def)
	}

	return tools
}
